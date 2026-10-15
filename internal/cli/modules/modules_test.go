package modules

import (
	"context"
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

func TestResolveTransitiveDependencies(t *testing.T) {
	cfg := &model.Config{
		Modules: []*model.Module{
			{
				Name: "org/service-a",
				Tag:  "v1.0.0",
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
				{Name: "org/common", Tag: "v2.0.0", DependencyType: "direct"},
				{Name: "org/indirect", Tag: "v1.0.0", DependencyType: "transitive"},
			},
		},
	}

	additional := resolveTransitiveDependencies(cfg, client)
	require.Len(t, additional, 1)
	assert.Equal(t, "org/common", additional[0].Name)
	assert.Equal(t, "v2.0.0", additional[0].Tag)
}
