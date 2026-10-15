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
	outDir := filepath.Join(tmpDir, "vendor")

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

	files, err := CollectProtoFilesInDirs([]string{"proto"})
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "proto/v1/a.proto", files[0].Filename)

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

	conn := NewInsecureConn(&model.Config{Registry: model.Registry{Addr: "localhost:6777", Insecure: true}}, parsedNrc)
	require.NotNil(t, conn)
	_ = conn.Close()
}
