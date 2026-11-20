package registry

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jdx/go-netrc"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/pbufio/pbuf-registry/internal/cli/patcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type stubRegistryClient struct {
	v1.RegistryClient
	pullResp *v1.PullModuleResponse
	pullErr  error
}

func (s *stubRegistryClient) PullModule(_ context.Context, _ *v1.PullModuleRequest, _ ...grpc.CallOption) (*v1.PullModuleResponse, error) {
	return s.pullResp, s.pullErr
}

func TestVendorRegistryModule(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	outDir := "vendor"

	client := &stubRegistryClient{
		pullResp: &v1.PullModuleResponse{
			Protofiles: []*v1.ProtoFile{
				{
					Filename: "api/v1/user.proto",
					Content:  "syntax = \"proto3\";\npackage api.v1;\noption go_package = \"old/pkg;v1\";\n",
				},
				{
					Filename: "api/v10/other.proto",
					Content:  "syntax = \"proto3\";\n",
				},
				{
					Filename: "../../etc/evil.proto",
					Content:  "evil",
				},
				{
					Filename: "root.proto",
					Content:  "syntax = \"proto3\";\n",
				},
				{
					Filename: "go.mod",
					Content:  "module evil\n",
				},
			},
		},
	}

	mod := &model.Module{
		Name:                 "org/users",
		Tag:                  "v1.0.0",
		Path:                 "api/v1",
		OutputFolder:         outDir,
		GenerateOutputFolder: "gen/api/v1",
	}

	err := VendorRegistryModule(mod, client, []patcher.Patcher{patcher.NewGoPackagePatcher("github.com/example/app")})
	require.NoError(t, err)

	// api/v1/user.proto should be written and patched
	written, err := os.ReadFile(filepath.Join(outDir, "user.proto"))
	require.NoError(t, err)
	assert.Contains(t, string(written), "github.com/example/app/gen/api/v1")

	// api/v10/other.proto should NOT match prefix api/v1
	_, err = os.Stat(filepath.Join(outDir, "other.proto"))
	assert.True(t, os.IsNotExist(err))

	// Root-level file selection (path: root.proto) writes to <outDir>/root.proto
	rootMod := &model.Module{
		Name:         "org/users",
		Tag:          "v1.0.0",
		Path:         "root.proto",
		OutputFolder: outDir,
	}
	require.NoError(t, VendorRegistryModule(rootMod, client, nil))
	_, err = os.Stat(filepath.Join(outDir, "root.proto"))
	require.NoError(t, err)

	// Unfiltered vendoring skips non-.proto files such as go.mod
	unfilteredMod := &model.Module{
		Name:         "org/users",
		Tag:          "v1.0.0",
		OutputFolder: outDir,
	}
	require.NoError(t, VendorRegistryModule(unfilteredMod, client, nil))
	_, err = os.Stat(filepath.Join(outDir, "go.mod"))
	assert.True(t, os.IsNotExist(err))

	// Reject absolute or traversal output folders and traversal module paths
	require.ErrorContains(t, VendorRegistryModule(&model.Module{
		Name:         "org/users",
		Tag:          "v1.0.0",
		OutputFolder: filepath.Join(t.TempDir(), "outside"),
	}, client, nil), "must be relative to workspace")
	require.ErrorContains(t, VendorRegistryModule(&model.Module{
		Name:         "org/users",
		Tag:          "v1.0.0",
		OutputFolder: "../outside",
	}, client, nil), "must stay within workspace")
	require.ErrorContains(t, VendorRegistryModule(&model.Module{
		Name: "org/users",
		Tag:  "v1.0.0",
		Path: "../outside",
	}, client, nil), "must not traverse outside module root")

	// Reject writing through symlinked output directory
	outsideDir := t.TempDir()
	require.NoError(t, os.Symlink(outsideDir, "symlink-vendor"))
	require.ErrorContains(t, VendorRegistryModule(&model.Module{
		Name:         "org/users",
		Tag:          "v1.0.0",
		Path:         "api/v1",
		OutputFolder: "symlink-vendor",
	}, client, nil), "refusing to write vendored proto through symlink")
}

func TestCollectProtoFilesInDirs(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	require.NoError(t, os.MkdirAll("proto/v1", 0o755))
	require.NoError(t, os.WriteFile("proto/v1/a.proto", []byte("syntax = \"proto3\";"), 0o644))
	require.NoError(t, os.WriteFile("proto/v1/readme.txt", []byte("ignore"), 0o644))
	secretPath := filepath.Join(t.TempDir(), "secret.proto")
	require.NoError(t, os.WriteFile(secretPath, []byte("syntax = \"proto3\"; message Secret {}"), 0o644))
	require.NoError(t, os.Symlink(secretPath, "proto/v1/symlink.proto"))

	files, err := CollectProtoFilesInDirs([]string{"./proto"})
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "proto/v1/a.proto", files[0].Filename)

	// Reject escaping or absolute export paths
	_, err = CollectProtoFilesInRoot(tmpDir, []string{"../outside"})
	require.ErrorContains(t, err, "must be relative to project root")
	_, err = CollectProtoFilesInRoot(tmpDir, []string{filepath.Join(tmpDir, "proto")})
	require.ErrorContains(t, err, "must be relative to project root")

	// Non-existent directory returns error without panicking
	_, err = CollectProtoFilesInDirs([]string{"nonexistent-dir"})
	require.Error(t, err)
}

func TestClientAddrAndNetrcHelpers(t *testing.T) {
	assert.Equal(t, "registry.example.com:6777", canonicalizeAddr("registry.example.com"))
	assert.Equal(t, "registry.example.com:9090", canonicalizeAddr("registry.example.com:9090"))
	assert.Equal(t, "registry.example.com", getRegistryHost("registry.example.com:6777"))
	assert.Equal(t, "registry.example.com", getRegistryHost("registry.example.com"))

	assert.Nil(t, credentialsFromNetrc("localhost", nil))

	netrcPath := filepath.Join(t.TempDir(), ".netrc")
	nrc := &netrc.Netrc{Path: netrcPath}
	nrc.AddMachine("localhost", "unused", "unused")
	nrc.Machine("localhost").Set("token", "secret-tok")
	require.NoError(t, nrc.Save())

	parsedNrc, err := netrc.Parse(netrcPath)
	require.NoError(t, err)

	creds := credentialsFromNetrc("localhost", parsedNrc)
	require.NotNil(t, creds)
	assert.False(t, creds.RequireTransportSecurity())
	md, err := creds.GetRequestMetadata(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "secret-tok", md["authorization"])

	// Standard .netrc format (`machine <host> login user password <token>`) without custom `token` key
	stdNetrcPath := filepath.Join(t.TempDir(), ".netrc")
	require.NoError(t, os.WriteFile(stdNetrcPath, []byte("machine pbuf.io login user password std-password-tok\nmachine empty.pbuf.io login user password unused\n"), 0o600))
	parsedStdNrc, err := netrc.Parse(stdNetrcPath)
	require.NoError(t, err)

	stdCreds := credentialsFromNetrc("pbuf.io", parsedStdNrc)
	require.NotNil(t, stdCreds)
	stdMD, err := stdCreds.GetRequestMetadata(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "std-password-tok", stdMD["authorization"])
	assert.Nil(t, credentialsFromNetrc("empty.pbuf.io", parsedStdNrc))

	conn, err := NewInsecureConn(&model.Config{Registry: model.Registry{Addr: "localhost:6777", Insecure: true}}, parsedNrc)
	require.NoError(t, err)
	require.NotNil(t, conn)
	_ = conn.Close()

	_, err = NewInsecureConn(&model.Config{Registry: model.Registry{Addr: "%zz:6777", Insecure: true}}, nil)
	require.ErrorContains(t, err, "failed to create grpc client")
}
