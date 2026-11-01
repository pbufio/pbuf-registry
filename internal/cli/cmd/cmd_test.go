package cmd

import (
	"bytes"
	"context"
	"encoding/json"
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
	pulledFiles   []*v1.ProtoFile
	listedModules []*v1.Module
	pages         map[string]*v1.ListModulesResponse
	listRequests  []*v1.ListModulesRequest
	emptyList     bool
	gotModule     *v1.Module
	gotDeps       []*v1.Dependency
}

func (f *fakeRegistryClient) PullModule(_ context.Context, _ *v1.PullModuleRequest, _ ...grpc.CallOption) (*v1.PullModuleResponse, error) {
	return &v1.PullModuleResponse{Protofiles: f.pulledFiles}, nil
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
	if f.gotModule != nil {
		return f.gotModule, nil
	}
	return &v1.Module{Name: in.Name, Tags: []string{"v1.2.0"}}, nil
}

func (f *fakeRegistryClient) GetModuleDependencies(_ context.Context, _ *v1.GetModuleDependenciesRequest, _ ...grpc.CallOption) (*v1.GetModuleDependenciesResponse, error) {
	return &v1.GetModuleDependenciesResponse{Dependencies: f.gotDeps}, nil
}

func (f *fakeRegistryClient) ListModules(_ context.Context, in *v1.ListModulesRequest, _ ...grpc.CallOption) (*v1.ListModulesResponse, error) {
	f.listRequests = append(f.listRequests, in)
	if f.emptyList {
		return &v1.ListModulesResponse{}, nil
	}
	if f.pages != nil {
		if resp, ok := f.pages[in.GetPageToken()]; ok {
			return resp, nil
		}
		return &v1.ListModulesResponse{}, nil
	}
	if f.listedModules != nil {
		return &v1.ListModulesResponse{Modules: f.listedModules}, nil
	}
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

	initDomainCmd := CreateInitCmd()
	initDomainCmd.SetArgs([]string{"org/hello", "--domain", "https://pbuf.io/"})
	require.NoError(t, initDomainCmd.Execute())

	rawDomain, err := os.ReadFile("pbuf.yaml")
	require.NoError(t, err)
	assert.Contains(t, string(rawDomain), "addr: pbuf.io")

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
		{"register", "org/hello"},
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

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(t.TempDir()))
	t.Setenv("PBUF_REGISTRY_ADDR", "localhost:6777")
	t.Setenv("PBUF_REGISTRY_INSECURE", "true")
	rootCmd := NewRootCmd()
	foundMCP := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "mcp" {
			foundMCP = true
		}
	}
	assert.True(t, foundMCP)
}

func TestDriftDiffCommand(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	require.NoError(t, os.MkdirAll("api", 0o755))
	require.NoError(t, os.WriteFile("api/pay.proto", []byte(`syntax = "proto3";
package acme.pay.v1;
enum Currency {
  CURRENCY_UNSPECIFIED = 0;
}
message Money {
  string units = 1;
}
service PayService {
  rpc StreamPay(stream Money) returns (Money);
}
`), 0o644))

	// Omit Export.Paths to verify default path resolution works when only api/ exists
	cfg := &model.Config{
		Name: "acme/payment",
	}
	regClient := &fakeRegistryClient{
		pulledFiles: []*v1.ProtoFile{
			{
				Filename: "api/pay.proto",
				Content: `syntax = "proto3";
package acme.pay.v1;
enum Currency {
  CURRENCY_UNSPECIFIED = 0;
  CURRENCY_USD = 1;
}
message Money {
  int64 units = 1;
  int32 nanos = 2;
}
service PayService {
  rpc StreamPay(Money) returns (Money);
}
`,
			},
		},
	}

	driftCmd := NewDriftCmd(cfg, &fakeDriftClient{}, regClient)
	var mdOut bytes.Buffer
	driftCmd.SetOut(&mdOut)
	driftCmd.SetArgs([]string{"diff", "--format", "markdown", "--fail-on", "none"})
	require.NoError(t, driftCmd.Execute())
	assert.Contains(t, mdOut.String(), "FIELD_TYPE_CHANGED")
	assert.Contains(t, mdOut.String(), "FIELD_REMOVED")
	assert.Contains(t, mdOut.String(), "ENUM_VALUE_REMOVED")
	assert.Contains(t, mdOut.String(), "(Money)->(Money) to (stream Money)->(Money)")

	// When --fail-on=critical (default), breaking changes must return an error
	driftCmdFail := NewDriftCmd(cfg, &fakeDriftClient{}, regClient)
	var tblOut bytes.Buffer
	driftCmdFail.SetOut(&tblOut)
	driftCmdFail.SetArgs([]string{"diff"})
	err = driftCmdFail.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CRITICAL")

	// Invalid --fail-on value must fail instead of silently passing
	driftCmdBadFlag := NewDriftCmd(cfg, &fakeDriftClient{}, regClient)
	driftCmdBadFlag.SetArgs([]string{"diff", "--fail-on", "typo"})
	require.ErrorContains(t, driftCmdBadFlag.Execute(), "invalid --fail-on")
}

func TestModulesListAndGetOutputFormats(t *testing.T) {
	cfg := &model.Config{Name: "acme/payment"}
	populatedClient := &fakeRegistryClient{
		listedModules: []*v1.Module{
			{
				Id:       "mod-1",
				Name:     "acme/payment",
				Tags:     []string{"v1.2.0", "v1.1.0"},
				Packages: []string{"acme.pay.v1"},
			},
			{
				Id:   "mod-2",
				Name: "acme/empty",
			},
		},
		gotModule: &v1.Module{
			Id:        "mod-1",
			Name:      "acme/payment",
			Tags:      []string{"v1.2.0", "v1.1.0"},
			DraftTags: []string{"v1.3.0-rc1"},
			Packages:  []string{"acme.pay.v1"},
		},
		gotDeps: []*v1.Dependency{
			{Name: "googleapis/googleapis", Tag: "v1.0.0", DependencyType: "direct"},
		},
	}

	tests := []struct {
		name        string
		client      *fakeRegistryClient
		config      *model.Config
		args        []string
		wantSubstrs []string
		wantJSON    bool
		wantErr     string
	}{
		{
			name:        "list default table",
			client:      populatedClient,
			config:      cfg,
			args:        []string{"list"},
			wantSubstrs: []string{"NAME", "ID", "LATEST TAG", "TAGS", "acme/payment", "mod-1", "v1.2.0", "v1.2.0, v1.1.0", "acme/empty", "mod-2"},
		},
		{
			name:        "list single page with page-size and page-token",
			client: &fakeRegistryClient{
				pages: map[string]*v1.ListModulesResponse{
					"tok-1": {
						Modules:       []*v1.Module{{Id: "m-2", Name: "acme/two", Tags: []string{"v2.0.0"}}},
						NextPageToken: "tok-2",
					},
				},
			},
			config:      cfg,
			args:        []string{"list", "--page-size", "1", "--page-token", "tok-1"},
			wantSubstrs: []string{"acme/two", "v2.0.0", "Next page token: tok-2"},
		},
		{
			name: "list --all aggregates all pages",
			client: &fakeRegistryClient{
				pages: map[string]*v1.ListModulesResponse{
					"": {
						Modules:       []*v1.Module{{Id: "m-1", Name: "acme/one", Tags: []string{"v1.0.0"}}},
						NextPageToken: "tok-2",
					},
					"tok-2": {
						Modules: []*v1.Module{{Id: "m-2", Name: "acme/two", Tags: []string{"v2.0.0"}}},
					},
				},
			},
			config:      cfg,
			args:        []string{"list", "--all", "--page-size", "1", "-o", "json"},
			wantSubstrs: []string{`"acme/one"`, `"acme/two"`},
			wantJSON:    true,
		},
		{
			name:    "list invalid non-positive page-size",
			client:  populatedClient,
			config:  cfg,
			args:    []string{"list", "--page-size", "0"},
			wantErr: "--page-size must be positive",
		},
		{
			name:        "list empty table",
			client:      &fakeRegistryClient{emptyList: true},
			config:      cfg,
			args:        []string{"list"},
			wantSubstrs: []string{"No modules found."},
		},
		{
			name:        "list json shorthand",
			client:      populatedClient,
			config:      cfg,
			args:        []string{"list", "-o", "json"},
			wantSubstrs: []string{`"modules"`, `"acme/payment"`},
			wantJSON:    true,
		},
		{
			name:        "get default table with dependencies",
			client:      populatedClient,
			config:      cfg,
			args:        []string{"get"},
			wantSubstrs: []string{"NAME:", "acme/payment", "ID:", "mod-1", "TAGS:", "v1.2.0, v1.1.0", "DRAFT TAGS:", "v1.3.0-rc1", "DEPENDENCY", "googleapis/googleapis", "direct"},
		},
		{
			name:        "get table without dependencies",
			client:      &fakeRegistryClient{},
			config:      cfg,
			args:        []string{"get", "acme/other"},
			wantSubstrs: []string{"NAME:", "acme/other", "Dependencies: none"},
		},
		{
			name:        "get json output",
			client:      populatedClient,
			config:      cfg,
			args:        []string{"get", "--output", "json"},
			wantSubstrs: []string{`"module"`, `"dependencies"`, `"googleapis/googleapis"`},
			wantJSON:    true,
		},
		{
			name:    "list invalid output format",
			client:  populatedClient,
			config:  cfg,
			args:    []string{"list", "--output", "xml"},
			wantErr: "invalid --output",
		},
		{
			name:    "get invalid output format",
			client:  populatedClient,
			config:  cfg,
			args:    []string{"get", "-o", "yaml"},
			wantErr: "invalid --output",
		},
		{
			name:    "get missing module name",
			client:  populatedClient,
			config:  &model.Config{},
			args:    []string{"get"},
			wantErr: "module name is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			modCmd := NewModuleCmd(tc.config, tc.client)
			var out bytes.Buffer
			modCmd.SetOut(&out)
			modCmd.SetErr(&out)
			modCmd.SetArgs(tc.args)

			err := modCmd.Execute()
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			got := out.String()
			for _, sub := range tc.wantSubstrs {
				assert.Contains(t, got, sub)
			}
			if tc.wantJSON {
				var parsed map[string]any
				require.NoError(t, json.Unmarshal(out.Bytes(), &parsed))
			}
		})
	}
}
