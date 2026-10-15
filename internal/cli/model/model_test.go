package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConfigHasRegistryAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	cfg := &Config{
		Version: "v1",
		Name:    "org/mod",
	}
	assert.False(t, cfg.HasRegistry())

	cfg.Registry.Addr = "localhost:6777"
	cfg.Registry.Insecure = true
	cfg.Modules = []*Module{
		{Name: "dep/one", Tag: "v1.0.0"},
	}
	assert.True(t, cfg.HasRegistry())

	require.NoError(t, cfg.Save())

	raw, err := os.ReadFile(filepath.Join(tmpDir, PbufConfigFilename))
	require.NoError(t, err)

	var loaded Config
	require.NoError(t, yaml.Unmarshal(raw, &loaded))
	assert.Equal(t, "org/mod", loaded.Name)
	assert.Equal(t, "localhost:6777", loaded.Registry.Addr)
	require.Len(t, loaded.Modules, 1)
	assert.Equal(t, "dep/one", loaded.Modules[0].Name)
}
