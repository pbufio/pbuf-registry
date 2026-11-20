package charts

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type chartMetadata struct {
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels"`
}

type chartResource struct {
	APIVersion string        `yaml:"apiVersion"`
	Kind       string        `yaml:"kind"`
	Metadata   chartMetadata `yaml:"metadata"`
	Spec       struct {
		MinAvailable   any `yaml:"minAvailable"`
		MaxUnavailable any `yaml:"maxUnavailable"`
		Selector       struct {
			MatchLabels map[string]string `yaml:"matchLabels"`
		} `yaml:"selector"`
		Template struct {
			Metadata chartMetadata `yaml:"metadata"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

func TestPodDisruptionBudget(t *testing.T) {
	helm, err := exec.LookPath("helm")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("Helm is required in CI for chart rendering tests")
		}
		t.Skip("Helm is required for chart rendering tests")
	}

	tests := []struct {
		name    string
		args    []string
		values  string
		enabled bool
		min     any
		max     any
		wantErr string
	}{
		{name: "disabled by default"},
		{name: "null parent map upgrade safety", args: []string{"--set", "podDisruptionBudget=null"}},
		{name: "default minimum and selector isolation", enabled: true, args: []string{"--set", "podLabels.team=platform,fullnameOverride=custom-release"}, min: 1},
		{name: "empty strings fall back to default", enabled: true, values: "podDisruptionBudget:\n  minAvailable: \"\"\n  maxUnavailable: \"\"\n", min: 1},
		{name: "zero maximum in values file", enabled: true, values: "podDisruptionBudget:\n  maxUnavailable: 0\n", max: 0},
		{name: "numeric string coerced to integer", enabled: true, values: "podDisruptionBudget:\n  minAvailable: \"2\"\n", min: 2},
		{name: "percentage minimum", enabled: true, values: "podDisruptionBudget:\n  minAvailable: \"75%\"\n", min: "75%"},
		{name: "conflicting min and max", enabled: true, args: []string{"--set", "podDisruptionBudget.minAvailable=1,podDisruptionBudget.maxUnavailable=0"}, wantErr: "set only one of podDisruptionBudget.minAvailable or podDisruptionBudget.maxUnavailable"},
		{name: "negative budget rejected", enabled: true, values: "podDisruptionBudget:\n  minAvailable: -1\n", wantErr: "invalid podDisruptionBudget.minAvailable"},
		{name: "fractional budget rejected", enabled: true, values: "podDisruptionBudget:\n  minAvailable: 1.5\n", wantErr: "invalid podDisruptionBudget.minAvailable"},
		{name: "out of range percentage rejected", enabled: true, values: "podDisruptionBudget:\n  maxUnavailable: \"150%\"\n", wantErr: "invalid podDisruptionBudget.maxUnavailable"},
		{name: "boolean budget rejected", enabled: true, values: "podDisruptionBudget:\n  minAvailable: true\n", wantErr: "invalid podDisruptionBudget.minAvailable"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			args := []string{"template", "pdb-test", "./pbuf-registry", "--set", "ui.enabled=true"}
			if tc.values != "" {
				valuesFile := filepath.Join(t.TempDir(), "values.yaml")
				require.NoError(t, os.WriteFile(valuesFile, []byte(tc.values), 0o600))
				args = append(args, "--values", valuesFile)
			}
			if tc.enabled {
				args = append(args, "--set", "podDisruptionBudget.enabled=true")
			}
			args = append(args, tc.args...)
			output, err := exec.Command(helm, args...).CombinedOutput()
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, string(output), tc.wantErr)
				return
			}
			require.NoError(t, err, "%s", output)
			decoder := yaml.NewDecoder(bytes.NewReader(output))
			var budgets, workloads []chartResource
			for {
				var resource chartResource
				err := decoder.Decode(&resource)
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				switch resource.Kind {
				case "PodDisruptionBudget":
					budgets = append(budgets, resource)
				case "Deployment", "StatefulSet":
					workloads = append(workloads, resource)
				}
			}
			if !tc.enabled {
				assert.Empty(t, budgets)
				return
			}
			require.Len(t, budgets, 1)
			pdb := budgets[0]
			assert.Equal(t, "policy/v1", pdb.APIVersion)
			assert.Equal(t, tc.min, pdb.Spec.MinAvailable)
			assert.Equal(t, tc.max, pdb.Spec.MaxUnavailable)
			require.Len(t, pdb.Spec.Selector.MatchLabels, 2)
			require.Len(t, workloads, 5)
			selected := 0
			for _, workload := range workloads {
				matches := true
				for key, value := range pdb.Spec.Selector.MatchLabels {
					if workload.Spec.Template.Metadata.Labels[key] != value {
						matches = false
					}
				}
				if matches {
					selected++
					assert.Equal(t, "Deployment", workload.Kind)
					assert.Equal(t, workload.Metadata.Name, pdb.Metadata.Name)
					assert.Equal(t, workload.Metadata.Labels, pdb.Metadata.Labels)
					assert.Equal(t, workload.Spec.Selector.MatchLabels, pdb.Spec.Selector.MatchLabels)
				}
			}
			assert.Equal(t, 1, selected, "PDB must select only the registry server")
		})
	}
}

func TestHelmChartPostgreSQLWorkersAndOIDC(t *testing.T) {
	helm, err := exec.LookPath("helm")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("Helm is required in CI for chart rendering tests")
		}
		t.Skip("Helm is required for chart rendering tests")
	}

	rawValues := `postgresql:
  auth:
    username: "pbuf:admin@org"
    password: "s3cr3t@p@ss:w/rd?a+b c#1%2"
    database: "pbuf_db"
service:
  grpc:
    auth:
      enabled: true
      type: oidc
      oidc:
        issuerUrl: "https://idp.example.com"
        clientId: "pbuf-grpc"
        adminGroups:
          - platform-admins
          - sre
  http:
    auth:
      enabled: true
      type: oidc
      oidc:
        issuerUrl: "https://idp.example.com"
        clientId: "pbuf-http"
        adminGroups: "ops,sec"
`
	valuesFile := filepath.Join(t.TempDir(), "values.yaml")
	require.NoError(t, os.WriteFile(valuesFile, []byte(rawValues), 0o600))

	output, err := exec.Command(helm, "template", "chart-test", "./pbuf-registry", "--values", valuesFile).CombinedOutput()
	require.NoError(t, err, "%s", output)

	type envVar struct {
		Name  string `yaml:"name"`
		Value string `yaml:"value"`
	}
	type containerSpec struct {
		Name            string   `yaml:"name"`
		Image           string   `yaml:"image"`
		Command         []string `yaml:"command"`
		Env             []envVar `yaml:"env"`
		SecurityContext struct {
			RunAsUser    int  `yaml:"runAsUser"`
			RunAsNonRoot bool `yaml:"runAsNonRoot"`
		} `yaml:"securityContext"`
		LivenessProbe struct {
			Exec struct {
				Command []string `yaml:"command"`
			} `yaml:"exec"`
			HTTPGet struct {
				Path string `yaml:"path"`
			} `yaml:"httpGet"`
		} `yaml:"livenessProbe"`
		ReadinessProbe struct {
			TimeoutSeconds int `yaml:"timeoutSeconds"`
			HTTPGet        struct {
				Path string `yaml:"path"`
			} `yaml:"httpGet"`
		} `yaml:"readinessProbe"`
	}
	type doc struct {
		Kind     string        `yaml:"kind"`
		Metadata chartMetadata `yaml:"metadata"`
		Data     map[string]string
		Spec     struct {
			Template struct {
				Spec struct {
					SecurityContext struct {
						FSGroup int `yaml:"fsGroup"`
					} `yaml:"securityContext"`
					InitContainers []containerSpec `yaml:"initContainers"`
					Containers     []containerSpec `yaml:"containers"`
				} `yaml:"spec"`
			} `yaml:"template"`
		} `yaml:"spec"`
	}

	decoder := yaml.NewDecoder(bytes.NewReader(output))
	var (
		foundPostgres bool
		foundSecret   bool
		workerInits   = map[string]bool{}
		serverEnv     = map[string]string{}
	)
	for {
		var d doc
		decodeErr := decoder.Decode(&d)
		if decodeErr == io.EOF {
			break
		}
		require.NoError(t, decodeErr)

		switch {
		case d.Kind == "StatefulSet" && d.Metadata.Name == "chart-test-pbuf-registry-postgresql":
			foundPostgres = true
			assert.Equal(t, 70, d.Spec.Template.Spec.SecurityContext.FSGroup)
			require.Len(t, d.Spec.Template.Spec.Containers, 1)
			pg := d.Spec.Template.Spec.Containers[0]
			assert.Equal(t, "postgres:18.1-alpine3.23", pg.Image)
			assert.Equal(t, 70, pg.SecurityContext.RunAsUser)
			assert.True(t, pg.SecurityContext.RunAsNonRoot)
			assert.Equal(t, []string{"pg_isready", "-U", "pbuf:admin@org", "-d", "pbuf_db"}, pg.LivenessProbe.Exec.Command)

		case d.Kind == "Secret" && d.Metadata.Name == "chart-test-pbuf-registry":
			foundSecret = true
			dsnBytes, b64Err := base64.StdEncoding.DecodeString(d.Data["DATA_DATABASE_DSN"])
			require.NoError(t, b64Err)
			parsedDSN, urlErr := url.Parse(string(dsnBytes))
			require.NoError(t, urlErr)
			assert.Equal(t, "pbuf:admin@org", parsedDSN.User.Username())
			pw, hasPW := parsedDSN.User.Password()
			require.True(t, hasPW)
			assert.Equal(t, "s3cr3t@p@ss:w/rd?a+b c#1%2", pw)
			assert.Equal(t, "/pbuf_db", parsedDSN.Path)

		case d.Kind == "Deployment" && (d.Metadata.Name == "chart-test-pbuf-registry-compaction" || d.Metadata.Name == "chart-test-pbuf-registry-protoparser"):
			require.Len(t, d.Spec.Template.Spec.InitContainers, 1)
			assert.Equal(t, "migrations", d.Spec.Template.Spec.InitContainers[0].Name)
			assert.Equal(t, []string{"/app/pbuf-migrations"}, d.Spec.Template.Spec.InitContainers[0].Command)
			require.Len(t, d.Spec.Template.Spec.Containers, 1)
			assert.Equal(t, "/healthz", d.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Path)
			assert.Equal(t, "/readyz", d.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Path)
			assert.Equal(t, 3, d.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds)
			workerInits[d.Metadata.Name] = true

		case d.Kind == "Deployment" && d.Metadata.Name == "chart-test-pbuf-registry":
			require.Len(t, d.Spec.Template.Spec.Containers, 1)
			assert.Equal(t, "/healthz", d.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Path)
			assert.Equal(t, "/readyz", d.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Path)
			assert.Equal(t, 3, d.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds)
			for _, e := range d.Spec.Template.Spec.Containers[0].Env {
				serverEnv[e.Name] = e.Value
			}
		}
	}

	assert.True(t, foundPostgres, "expected postgresql StatefulSet")
	assert.True(t, foundSecret, "expected registry Secret")
	assert.True(t, workerInits["chart-test-pbuf-registry-compaction"], "compaction must run migrations initContainer")
	assert.True(t, workerInits["chart-test-pbuf-registry-protoparser"], "protoparser must run migrations initContainer")
	assert.Equal(t, "platform-admins,sre", serverEnv["SERVER_GRPC_AUTH_OIDC_ADMINGROUPS"])
	assert.Equal(t, "ops,sec", serverEnv["SERVER_HTTP_AUTH_OIDC_ADMINGROUPS"])
}
