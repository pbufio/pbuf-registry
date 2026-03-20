package server

import (
	"context"
	"testing"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetModuleDependencies_DirectOnly(t *testing.T) {
	registryRepository := &mocks.RegistryRepository{}
	metadataRepository := &mocks.MetadataRepository{}

	directDeps := []*v1.Dependency{
		{Name: "pkg/b", Tag: "v1.0"},
	}
	registryRepository.On("GetModuleDependencies", mock.Anything, "pkg/a", "v1.0").Return(directDeps, nil)

	server := NewRegistryServer(registryRepository, metadataRepository, nil)

	resp, err := server.GetModuleDependencies(context.Background(), &v1.GetModuleDependenciesRequest{
		Name:               "pkg/a",
		Tag:                "v1.0",
		ResolveTransitive:  false,
	})

	assert.NoError(t, err)
	assert.Len(t, resp.Dependencies, 1)
	assert.Equal(t, "pkg/b", resp.Dependencies[0].Name)
	assert.Equal(t, "v1.0", resp.Dependencies[0].Tag)
}

func TestGetModuleDependencies_WithTransitive(t *testing.T) {
	registryRepository := &mocks.RegistryRepository{}
	metadataRepository := &mocks.MetadataRepository{}

	transitiveDeps := []*v1.Dependency{
		{Name: "pkg/b", Tag: "v1.0", DependencyType: "direct"},
		{Name: "pkg/c", Tag: "v2.0", DependencyType: "transitive"},
	}
	registryRepository.On("GetTransitiveDependencies", mock.Anything, "pkg/a", "v1.0").Return(transitiveDeps, nil)

	server := NewRegistryServer(registryRepository, metadataRepository, nil)

	resp, err := server.GetModuleDependencies(context.Background(), &v1.GetModuleDependenciesRequest{
		Name:               "pkg/a",
		Tag:                "v1.0",
		ResolveTransitive:  true,
	})

	assert.NoError(t, err)
	assert.Len(t, resp.Dependencies, 2)
	assert.Equal(t, "pkg/b", resp.Dependencies[0].Name)
	assert.Equal(t, "direct", resp.Dependencies[0].DependencyType)
	assert.Equal(t, "pkg/c", resp.Dependencies[1].Name)
	assert.Equal(t, "transitive", resp.Dependencies[1].DependencyType)
}

func TestGetModuleDependencies_EmptyName(t *testing.T) {
	registryRepository := &mocks.RegistryRepository{}
	metadataRepository := &mocks.MetadataRepository{}

	server := NewRegistryServer(registryRepository, metadataRepository, nil)

	_, err := server.GetModuleDependencies(context.Background(), &v1.GetModuleDependenciesRequest{
		Name: "",
		Tag:  "v1.0",
	})

	assert.Error(t, err)
	assert.Equal(t, "name cannot be empty", err.Error())
}
