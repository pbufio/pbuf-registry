package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/pbufio/pbuf-registry/internal/cli/patcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVendorGitModule(t *testing.T) {
	repoDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "vendor")

	repo, err := gogit.PlainInit(repoDir, false)
	require.NoError(t, err)

	protoDir := filepath.Join(repoDir, "api", "v1")
	require.NoError(t, os.MkdirAll(protoDir, 0o755))
	protoFile := filepath.Join(protoDir, "hello.proto")
	require.NoError(t, os.WriteFile(protoFile, []byte("syntax = \"proto3\";\npackage api.v1;\noption go_package = \"old/pkg;v1\";\n"), 0o644))
	rootProto := filepath.Join(repoDir, "root.proto")
	require.NoError(t, os.WriteFile(rootProto, []byte("syntax = \"proto3\";\n"), 0o644))

	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("api/v1/hello.proto")
	require.NoError(t, err)
	_, err = wt.Add("root.proto")
	require.NoError(t, err)

	commitHash, err := wt.Commit("initial proto", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	_, err = repo.CreateTag("v1.0.0", commitHash, nil)
	require.NoError(t, err)

	mod := &model.Module{
		Repository:           repoDir,
		Path:                 "api/v1",
		Tag:                  "v1.0.0",
		OutputFolder:         outDir,
		GenerateOutputFolder: "gen/api/v1",
	}

	err = VendorGitModule(mod, nil, []patcher.Patcher{patcher.NewGoPackagePatcher("github.com/example/app")})
	require.NoError(t, err)

	written, err := os.ReadFile(filepath.Join(outDir, "hello.proto"))
	require.NoError(t, err)
	assert.Contains(t, string(written), "github.com/example/app/gen/api/v1")

	rootMod := &model.Module{
		Repository:   repoDir,
		Path:         "root.proto",
		Tag:          "v1.0.0",
		OutputFolder: outDir,
	}
	require.NoError(t, VendorGitModule(rootMod, nil, nil))
	_, err = os.Stat(filepath.Join(outDir, "root.proto"))
	require.NoError(t, err)
}
