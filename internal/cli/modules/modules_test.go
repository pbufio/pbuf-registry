package modules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type fakeRegistryClient struct {
	v1.RegistryClient
	deps map[string][]*v1.Dependency
}

func (f *fakeRegistryClient) GetModuleDependencies(_ context.Context, in *v1.GetModuleDependenciesRequest, _ ...grpc.CallOption) (*v1.GetModuleDependenciesResponse, error) {
	return &v1.GetModuleDependenciesResponse{
		Dependencies: f.deps[in.Name+"@"+in.Tag],
	}, nil
}

func (f *fakeRegistryClient) PullModule(_ context.Context, in *v1.PullModuleRequest, _ ...grpc.CallOption) (*v1.PullModuleResponse, error) {
	return &v1.PullModuleResponse{
		Protofiles: []*v1.ProtoFile{
			{
				Filename: in.Name + ".proto",
				Content:  "syntax = \"proto3\";\n",
			},
		},
	}, nil
}

func TestResolveTransitiveDependencies(t *testing.T) {
	cfg := &model.Config{
		Name: "org/app",
		Modules: []*model.Module{
			{
				Name:                 "org/service-a",
				Tag:                  "v1.0.0",
				OutputFolder:         "third_party",
				GenerateOutputFolder: "gen",
			},
			{
				Name:                 "org/service-b",
				Tag:                  "v1.0.0",
				OutputFolder:         "./third_party/",
				GenerateOutputFolder: "gen",
			},
			{
				Name:         "org/configured",
				Tag:          "v1.0.0",
				OutputFolder: "third_party",
			},
			{
				Name:         "org/other-dir-configured",
				Tag:          "v1.0.0",
				OutputFolder: "vendor_other",
			},
			{
				Name:       "org/indirect",
				Repository: "https://github.com/acme/protos",
			},
			{
				Name: "buf.build/bufbuild/protovalidate",
				Tag:  "v0.14.0",
			},
		},
	}

	client := &fakeRegistryClient{
		deps: map[string][]*v1.Dependency{
			"org/service-a@v1.0.0": {
				{Name: "org/common", Tag: "v1.0.0", DependencyType: "direct"},
				{Name: "org/indirect", Tag: "v1.0.0", DependencyType: "transitive"},
				{Name: "org/configured", Tag: "v1.0.0", DependencyType: "direct"},
				{Name: "org/other-dir-configured", Tag: "v1.1.0", DependencyType: "direct"},
				{Name: "org/app", Tag: "v0.1.0", DependencyType: "transitive"},
			},
			"org/service-b@v1.0.0": {
				{Name: "org/common", Tag: "v1.2.0", DependencyType: "direct"},
			},
		},
	}

	additional := resolveTransitiveDependencies(cfg, client)
	require.Len(t, additional, 3)
	assert.Equal(t, "org/common", additional[0].Name)
	assert.Equal(t, "v1.2.0", additional[0].Tag)
	assert.Equal(t, "third_party", additional[0].OutputFolder)
	assert.Equal(t, "gen", additional[0].GenerateOutputFolder)
	assert.Equal(t, "org/indirect", additional[1].Name)
	assert.Equal(t, "v1.0.0", additional[1].Tag)
	assert.Equal(t, "third_party", additional[1].OutputFolder)
	assert.Equal(t, "gen", additional[1].GenerateOutputFolder)
	assert.Equal(t, "org/other-dir-configured", additional[2].Name)
	assert.Equal(t, "v1.1.0", additional[2].Tag)
	assert.Equal(t, "third_party", additional[2].OutputFolder)
}

func TestNewConfigAndVendor(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module github.com/acme/app\n\ngo 1.26\n"), 0o644))

	yamlBytes := []byte(`version: v1
name: org/app
registry:
  addr: localhost:6777
  insecure: true
modules:
  - name: org/service-a
    tag: v1.0.0
    out: vendor
    gen_out: gen
`)
	cfg, err := NewConfig(yamlBytes)
	require.NoError(t, err)
	assert.Equal(t, "org/app", cfg.Name)

	client := &fakeRegistryClient{
		deps: map[string][]*v1.Dependency{
			"org/service-a@v1.0.0": {
				{Name: "org/common", Tag: "v2.0.0", DependencyType: "direct"},
				{Name: "org/indirect", Tag: "v1.0.0", DependencyType: "transitive"},
			},
		},
	}

	require.NoError(t, Vendor(cfg, nil, client))
	for _, rel := range []string{
		filepath.Join("vendor", "org", "service-a.proto"),
		filepath.Join("vendor", "org", "common.proto"),
		filepath.Join("vendor", "org", "indirect.proto"),
	} {
		data, readErr := os.ReadFile(filepath.Join(tmpDir, rel))
		require.NoError(t, readErr)
		assert.Contains(t, string(data), `option go_package = "github.com/acme/app/gen/org;org";`)
	}
}
