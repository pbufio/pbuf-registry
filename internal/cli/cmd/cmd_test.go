package cmd

import (
	"bytes"
	"context"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/jdx/go-netrc"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type fakeRegistryClient struct {
	v1.RegistryClient
}

func (f *fakeRegistryClient) RegisterModule(_ context.Context, in *v1.RegisterModuleRequest, _ ...grpc.CallOption) (*v1.Module, error) {
	return &v1.Module{Name: in.Name}, nil
}

func (f *fakeRegistryClient) PushModule(_ context.Context, in *v1.PushModuleRequest, _ ...grpc.CallOption) (*v1.Module, error) {
	return &v1.Module{Name: in.ModuleName, Tags: []string{in.Tag}}, nil
}

func (f *fakeRegistryClient) DeleteModuleTag(_ context.Context, _ *v1.DeleteModuleTagRequest, _ ...grpc.CallOption) (*v1.DeleteModuleTagResponse, error) {
	return &v1.DeleteModuleTagResponse{}, nil
}

func (f *fakeRegistryClient) DeleteModule(_ context.Context, _ *v1.DeleteModuleRequest, _ ...grpc.CallOption) (*v1.DeleteModuleResponse, error) {
	return &v1.DeleteModuleResponse{}, nil
}

func (f *fakeRegistryClient) GetModule(_ context.Context, in *v1.GetModuleRequest, _ ...grpc.CallOption) (*v1.Module, error) {
	return &v1.Module{Name: in.Name, Tags: []string{"v1.2.0"}}, nil
}

func (f *fakeRegistryClient) GetModuleDependencies(_ context.Context, _ *v1.GetModuleDependenciesRequest, _ ...grpc.CallOption) (*v1.GetModuleDependenciesResponse, error) {
	return &v1.GetModuleDependenciesResponse{}, nil
}

func (f *fakeRegistryClient) ListModules(_ context.Context, _ *v1.ListModulesRequest, _ ...grpc.CallOption) (*v1.ListModulesResponse, error) {
	return &v1.ListModulesResponse{Modules: []*v1.Module{{Name: "org/mod"}}}, nil
}

type fakeUsersClient struct {
	v1.UserServiceClient
}

func (f *fakeUsersClient) CreateUser(_ context.Context, in *v1.CreateUserRequest, _ ...grpc.CallOption) (*v1.CreateUserResponse, error) {
	return &v1.CreateUserResponse{User: &v1.User{Id: "u-1", Name: in.Name, Type: in.Type}, Token: "tok"}, nil
}

func (f *fakeUsersClient) ListUsers(_ context.Context, _ *v1.ListUsersRequest, _ ...grpc.CallOption) (*v1.ListUsersResponse, error) {
	return &v1.ListUsersResponse{Users: []*v1.User{{Id: "u-1", Name: "alice"}}}, nil
}

func (f *fakeUsersClient) GetUser(_ context.Context, in *v1.GetUserRequest, _ ...grpc.CallOption) (*v1.User, error) {
	return &v1.User{Id: in.Id, Name: "alice", IsActive: true}, nil
}

func (f *fakeUsersClient) UpdateUser(_ context.Context, in *v1.UpdateUserRequest, _ ...grpc.CallOption) (*v1.User, error) {
	return &v1.User{Id: in.Id, Name: in.Name, IsActive: in.IsActive}, nil
}

func (f *fakeUsersClient) DeleteUser(_ context.Context, _ *v1.DeleteUserRequest, _ ...grpc.CallOption) (*v1.DeleteUserResponse, error) {
	return &v1.DeleteUserResponse{}, nil
}

func (f *fakeUsersClient) RegenerateToken(_ context.Context, _ *v1.RegenerateTokenRequest, _ ...grpc.CallOption) (*v1.RegenerateTokenResponse, error) {
	return &v1.RegenerateTokenResponse{Token: "new-tok"}, nil
}

func (f *fakeUsersClient) GrantPermission(_ context.Context, _ *v1.GrantPermissionRequest, _ ...grpc.CallOption) (*v1.GrantPermissionResponse, error) {
	return &v1.GrantPermissionResponse{}, nil
}

func (f *fakeUsersClient) RevokePermission(_ context.Context, _ *v1.RevokePermissionRequest, _ ...grpc.CallOption) (*v1.RevokePermissionResponse, error) {
	return &v1.RevokePermissionResponse{}, nil
}

func (f *fakeUsersClient) ListUserPermissions(_ context.Context, _ *v1.ListUserPermissionsRequest, _ ...grpc.CallOption) (*v1.ListUserPermissionsResponse, error) {
	return &v1.ListUserPermissionsResponse{}, nil
}

type fakeMetadataClient struct {
	v1.MetadataServiceClient
}

func (f *fakeMetadataClient) GetMetadata(_ context.Context, _ *v1.GetMetadataRequest, _ ...grpc.CallOption) (*v1.GetMetadataResponse, error) {
	return &v1.GetMetadataResponse{}, nil
}

type fakeDriftClient struct {
	v1.DriftServiceClient
}

func (f *fakeDriftClient) ListDriftEvents(_ context.Context, _ *v1.ListDriftEventsRequest, _ ...grpc.CallOption) (*v1.ListDriftEventsResponse, error) {
	return &v1.ListDriftEventsResponse{}, nil
}

func (f *fakeDriftClient) GetModuleDriftEvents(_ context.Context, _ *v1.GetModuleDriftEventsRequest, _ ...grpc.CallOption) (*v1.GetModuleDriftEventsResponse, error) {
	return &v1.GetModuleDriftEventsResponse{}, nil
}

func (f *fakeDriftClient) GetModuleDependencyDriftStatus(_ context.Context, _ *v1.GetModuleDependencyDriftStatusRequest, _ ...grpc.CallOption) (*v1.GetModuleDependencyDriftStatusResponse, error) {
	return &v1.GetModuleDependencyDriftStatusResponse{
		Statuses: []*v1.DependencyDriftStatus{
			{
				DependencyName: "org/dep",
				CurrentTag:     "v1.0.0",
				TargetTag:      "v1.1.0",
				Severity:       v1.DriftSeverity_DRIFT_SEVERITY_WARNING,
			},
		},
	}, nil
}

func (f *fakeDriftClient) AcknowledgeDriftEvent(_ context.Context, in *v1.AcknowledgeDriftEventRequest, _ ...grpc.CallOption) (*v1.AcknowledgeDriftEventResponse, error) {
	return &v1.AcknowledgeDriftEventResponse{
		Event: &v1.DriftEvent{
			Id:             in.EventId,
			Acknowledged:   true,
			AcknowledgedBy: in.AcknowledgedBy,
		},
	}, nil
}

func TestSaveTokenToNetrcAndAuthCmd(t *testing.T) {
	tmpDir := t.TempDir()
	usr := &user.User{HomeDir: tmpDir}
	cfg := &model.Config{
		Registry: model.Registry{Addr: "registry.example.com:6777"},
	}

	authCmd := NewAuthCmd(cfg, usr, nil)
	authCmd.SetArgs([]string{"my-secret-token"})
	require.NoError(t, authCmd.Execute())

	parsed, err := netrc.Parse(filepath.Join(tmpDir, ".netrc"))
	require.NoError(t, err)
	m := parsed.Machine("registry.example.com")
	require.NotNil(t, m)
	assert.Equal(t, "my-secret-token", m.Get("token"))
}

func TestInitAndModuleCommands(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	initCmd := CreateInitCmd()
	initCmd.SetArgs([]string{"org/hello", "localhost:6777"})
	require.NoError(t, initCmd.Execute())

	raw, err := os.ReadFile("pbuf.yaml")
	require.NoError(t, err)
	assert.Contains(t, string(raw), "org/hello")
	assert.Contains(t, string(raw), "localhost:6777")

	require.NoError(t, os.MkdirAll("api", 0o755))
	require.NoError(t, os.WriteFile("api/hello.proto", []byte("syntax = \"proto3\";"), 0o644))

	cfg := &model.Config{
		Name:   "org/hello",
		Export: model.Export{Paths: []string{"api"}},
		Modules: []*model.Module{
			{Name: "org/dep", Tag: "v1.0.0"},
		},
	}
	regClient := &fakeRegistryClient{}
	modCmd := NewModuleCmd(cfg, regClient)

	for _, args := range [][]string{
		{"register"},
		{"push", "v1.0.0"},
		{"get"},
		{"list"},
		{"update"},
		{"delete-tag", "v1.0.0"},
		{"delete"},
	} {
		modCmd.SetArgs(args)
		require.NoError(t, modCmd.Execute())
	}
	assert.Equal(t, "v1.2.0", cfg.Modules[0].Tag)
}

func TestUsersMetadataAndDriftCommands(t *testing.T) {
	cfg := &model.Config{Name: "org/hello"}

	usersCmd := NewUsersCmd(cfg, &fakeUsersClient{})
	for _, args := range [][]string{
		{"create", "alice", "--type", "user"},
		{"create", "ci-bot", "--type", "bot"},
		{"list"},
		{"get", "u-1"},
		{"update", "u-1", "--name", "alice-new"},
		{"update", "u-1", "--active"},
		{"regenerate-token", "u-1"},
		{"grant-permission", "u-1", "*", "--permission", "admin"},
		{"grant-permission", "u-1", "org/hello", "--permission", "write"},
		{"grant-permission", "u-1", "org/hello", "--permission", "read"},
		{"list-permissions", "u-1"},
		{"revoke-permission", "u-1", "org/hello"},
		{"delete", "u-1"},
	} {
		usersCmd.SetArgs(args)
		require.NoError(t, usersCmd.Execute())
	}

	metaCmd := NewMetadataCmd(cfg, &fakeMetadataClient{})
	metaCmd.SetArgs([]string{"get", "org/hello", "v1.0.0"})
	require.NoError(t, metaCmd.Execute())

	driftCmd := NewDriftCmd(cfg, &fakeDriftClient{})
	var out bytes.Buffer
	driftCmd.SetOut(&out)
	for _, args := range [][]string{
		{"list"},
		{"list", "--unacknowledged-only=false"},
		{"module", "org/hello", "--tag", "v1.0.0"},
		{"dependencies", "org/hello", "--tag", "v1.0.0"},
		{"acknowledge", "evt-123", "--by", "alice"},
	} {
		driftCmd.SetArgs(args)
		require.NoError(t, driftCmd.Execute())
	}
	assert.Contains(t, out.String(), "org/dep")

	mcpCmd := NewMCPCmd(&fakeRegistryClient{}, &fakeMetadataClient{}, &fakeDriftClient{})
	var mcpOut bytes.Buffer
	mcpCmd.SetIn(bytes.NewBufferString("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\"}\n"))
	mcpCmd.SetOut(&mcpOut)
	require.NoError(t, mcpCmd.Execute())
	assert.Contains(t, mcpOut.String(), `"list_modules"`)
}
