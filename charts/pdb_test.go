package charts

import (
	"bytes"
	"io"
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
		t.Skip("Helm is required for chart rendering tests")
	}

	tests := []struct {
		name    string
		args    []string
		values  string
		enabled bool
		min     any
		max     any
		wantErr bool
	}{
		{name: "disabled by default"},
		{name: "explicitly disabled", args: []string{"--set", "podDisruptionBudget.enabled=false"}},
		{name: "default minimum", enabled: true, min: 1},
		{name: "single replica", enabled: true, args: []string{"--set", "replicaCount=1"}, min: 1},
		{name: "zero minimum", enabled: true, args: []string{"--set", "podDisruptionBudget.minAvailable=0"}, min: 0},
		{name: "integer minimum", enabled: true, args: []string{"--set", "podDisruptionBudget.minAvailable=2"}, min: 2},
		{name: "percentage minimum", enabled: true, args: []string{"--set", "podDisruptionBudget.minAvailable=75%"}, min: "75%"},
		{name: "full percentage minimum", enabled: true, args: []string{"--set", "podDisruptionBudget.minAvailable=100%"}, min: "100%"},
		{name: "zero maximum", enabled: true, args: []string{"--set", "podDisruptionBudget.maxUnavailable=0"}, max: 0},
		{name: "integer maximum", enabled: true, args: []string{"--set", "podDisruptionBudget.maxUnavailable=1"}, max: 1},
		{name: "percentage maximum", enabled: true, args: []string{"--set", "podDisruptionBudget.maxUnavailable=25%"}, max: "25%"},
		{name: "zero percentage maximum", enabled: true, args: []string{"--set", "podDisruptionBudget.maxUnavailable=0%"}, max: "0%"},
		{name: "null minimum with maximum", enabled: true, args: []string{"--set", "podDisruptionBudget.minAvailable=null,podDisruptionBudget.maxUnavailable=0"}, max: 0},
		{name: "null maximum with minimum", enabled: true, args: []string{"--set", "podDisruptionBudget.maxUnavailable=null,podDisruptionBudget.minAvailable=2"}, min: 2},
		{name: "percentage in values file", enabled: true, values: "podDisruptionBudget:\n  minAvailable: \"50%\"\n", min: "50%"},
		{name: "zero maximum in values file", enabled: true, values: "podDisruptionBudget:\n  maxUnavailable: 0\n", max: 0},
		{name: "additional pod labels", enabled: true, args: []string{"--set", "podLabels.team=platform"}, min: 1},
		{name: "name override", enabled: true, args: []string{"--set", "nameOverride=custom-registry"}, min: 1},
		{name: "full name override", enabled: true, args: []string{"--set", "fullnameOverride=custom-release"}, min: 1},
		{name: "conflicting integers", enabled: true, args: []string{"--set", "podDisruptionBudget.minAvailable=1,podDisruptionBudget.maxUnavailable=1"}, wantErr: true},
		{name: "conflicting zero values", enabled: true, args: []string{"--set", "podDisruptionBudget.minAvailable=0,podDisruptionBudget.maxUnavailable=0"}, wantErr: true},
		{name: "conflicting percentage and integer", enabled: true, args: []string{"--set", "podDisruptionBudget.minAvailable=50%,podDisruptionBudget.maxUnavailable=0"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, string(output), "set only one of podDisruptionBudget.minAvailable or podDisruptionBudget.maxUnavailable")
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
			require.Len(t, workloads, 5) // Server, two workers, UI, and PostgreSQL.
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
