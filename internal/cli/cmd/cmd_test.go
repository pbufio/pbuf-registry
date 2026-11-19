package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
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
	assert.Equal(t, "my-secret-token", m.Get("password"))

	// Rotating an existing host entry updates both password and token
	rotateCmd := NewAuthCmd(cfg, usr, nil)
	rotateCmd.SetArgs([]string{"rotated-secret-token"})
	require.NoError(t, rotateCmd.Execute())

	parsedAfter, err := netrc.Parse(filepath.Join(tmpDir, ".netrc"))
	require.NoError(t, err)
	mAfter := parsedAfter.Machine("registry.example.com")
	require.NotNil(t, mAfter)
	assert.Equal(t, "rotated-secret-token", mAfter.Get("token"))
	assert.Equal(t, "rotated-secret-token", mAfter.Get("password"))
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
	assert.NotContains(t, string(raw), "pbufio/pbuf-registry")

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

	// Missing dependency tag fails before PushModule RPC
	badDepCfg := &model.Config{
		Name:   "org/hello",
		Export: model.Export{Paths: []string{"api"}},
		Modules: []*model.Module{
			{Name: "org/untagged-dep"},
		},
	}
	badModCmd := NewModuleCmd(badDepCfg, regClient)
	badModCmd.SetArgs([]string{"push", "v1.0.1"})
	err = badModCmd.Execute()
	require.ErrorContains(t, err, `dependency "org/untagged-dep" in pbuf.yaml is missing a tag`)
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

	// Verify `pbuf mcp` exposes and executes `diff_local_protos` against an explicit project directory
	extDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(extDir, "proto"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(extDir, "pbuf.yaml"), []byte("version: v1\nname: acme/orders\nexport:\n  paths:\n    - proto\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(extDir, "proto", "order.proto"), []byte("syntax = \"proto3\";\npackage acme.orders.v1;\nmessage Order {\n  string id = 1;\n}\n"), 0o644))

	mcpRegClient := &fakeRegistryClient{
		gotModule: &v1.Module{Name: "acme/orders", Tags: []string{"v1.0.0"}},
		pulledFiles: []*v1.ProtoFile{
			{
				Filename: "proto/order.proto",
				Content:  "syntax = \"proto3\";\npackage acme.orders.v1;\nmessage Order {\n  string id = 1;\n  int64 total_cents = 2;\n}\n",
			},
		},
	}
	mcpCmd := NewMCPCmd(mcpRegClient, &fakeMetadataClient{}, &fakeDriftClient{})
	callReq, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "diff_local_protos",
			"arguments": map[string]any{
				"directory": extDir,
			},
		},
	})
	require.NoError(t, err)
	mcpIn := "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\"}\n" + string(callReq) + "\n"
	var mcpOut bytes.Buffer
	mcpCmd.SetIn(bytes.NewBufferString(mcpIn))
	mcpCmd.SetOut(&mcpOut)
	require.NoError(t, mcpCmd.Execute())

	mcpLines := bytes.Split(bytes.TrimSpace(mcpOut.Bytes()), []byte("\n"))
	require.Len(t, mcpLines, 2)
	assert.Contains(t, string(mcpLines[0]), `"diff_local_protos"`)

	var callResp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(mcpLines[1], &callResp))
	require.Len(t, callResp.Result.Content, 1)

	var report LocalDriftDiffReport
	require.NoError(t, json.Unmarshal([]byte(callResp.Result.Content[0].Text), &report))
	assert.Equal(t, "acme/orders", report.ModuleName)
	assert.Equal(t, "v1.0.0", report.BaseTag)
	assert.True(t, report.HasBreakingChanges)
	require.Len(t, report.Items, 1)
	assert.Equal(t, "FIELD_REMOVED", report.Items[0].Change)

	// Verify fallbackCfg does not bleed into a separate target directory lacking pbuf.yaml
	emptyRepoDir := t.TempDir()
	fallbackCfg := &model.Config{Name: "acme/cwd-module", Export: model.Export{Paths: []string{"proto"}}}
	mcpIsolated := NewMCPCmd(mcpRegClient, &fakeMetadataClient{}, &fakeDriftClient{}, fallbackCfg)
	isolatedReq, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "diff_local_protos",
			"arguments": map[string]any{"directory": emptyRepoDir},
		},
	})
	require.NoError(t, err)
	var isolatedOut bytes.Buffer
	mcpIsolated.SetIn(bytes.NewBufferString(string(isolatedReq) + "\n"))
	mcpIsolated.SetOut(&isolatedOut)
	require.NoError(t, mcpIsolated.Execute())
	assert.Contains(t, isolatedOut.String(), `"isError":true`)
	assert.Contains(t, isolatedOut.String(), "module name is required")
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
			name: "list single page with page-size and page-token",
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
			name: "list --all detects multi-step token cycle",
			client: &fakeRegistryClient{
				pages: map[string]*v1.ListModulesResponse{
					"":      {Modules: []*v1.Module{{Name: "acme/a"}}, NextPageToken: "tok-b"},
					"tok-b": {Modules: []*v1.Module{{Name: "acme/b"}}, NextPageToken: "tok-a"},
					"tok-a": {Modules: []*v1.Module{{Name: "acme/a"}}, NextPageToken: "tok-b"},
				},
			},
			config:  cfg,
			args:    []string{"list", "--all"},
			wantErr: "pagination cycle detected",
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

func TestVersionCommand(t *testing.T) {
	t.Run("text output", func(t *testing.T) {
		versionCmd := NewVersionCmd()
		var out bytes.Buffer
		versionCmd.SetOut(&out)
		versionCmd.SetArgs(nil)
		require.NoError(t, versionCmd.Execute())

		got := out.String()
		for _, sub := range []string{"pbuf ", "commit:", "built:", "go:", "platform:"} {
			assert.Contains(t, got, sub)
		}
	})

	t.Run("json output", func(t *testing.T) {
		versionCmd := NewVersionCmd()
		var out bytes.Buffer
		versionCmd.SetOut(&out)
		versionCmd.SetArgs([]string{"-o", "json"})
		require.NoError(t, versionCmd.Execute())

		var parsed versionInfo
		require.NoError(t, json.Unmarshal(out.Bytes(), &parsed))
		assert.NotEmpty(t, parsed.Version)
		assert.NotEmpty(t, parsed.Commit)
		assert.NotEmpty(t, parsed.Date)
		assert.NotEmpty(t, parsed.GoVersion)
		assert.NotEmpty(t, parsed.Platform)
	})

	t.Run("invalid output format", func(t *testing.T) {
		versionCmd := NewVersionCmd()
		var out bytes.Buffer
		versionCmd.SetOut(&out)
		versionCmd.SetErr(&out)
		versionCmd.SetArgs([]string{"-o", "xml"})
		require.ErrorContains(t, versionCmd.Execute(), "invalid --output")
	})

	t.Run("root --version flag", func(t *testing.T) {
		origDir, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(origDir) }()
		require.NoError(t, os.Chdir(t.TempDir()))

		rootCmd := NewRootCmd()
		var out bytes.Buffer
		rootCmd.SetOut(&out)
		rootCmd.SetErr(&out)
		rootCmd.SetArgs([]string{"--version"})
		require.NoError(t, rootCmd.Execute())
		assert.Contains(t, out.String(), "pbuf ")
	})

	t.Run("build info falls back without ldflags", func(t *testing.T) {
		info := buildInfo()
		assert.NotEmpty(t, info.Version)
		assert.NotEmpty(t, info.Commit)
		assert.NotEmpty(t, info.Date)
		assert.NotEmpty(t, info.GoVersion)
	})
}

func TestLintCommand(t *testing.T) {
	const validProto = `syntax = "proto3";
package acme.pay.v1;
enum Currency {
  CURRENCY_UNSPECIFIED = 0;
  CURRENCY_USD = 1;
}
message Money {
  Currency currency = 1;
  int64 units = 2;
}
service PayService {
  rpc GetBalance(Money) returns (Money);
}
`

	const missingSyntaxProto = `package acme.pay.v1;
message Money {
  int64 units = 1;
}
`

	const brokenProto = `syntax = "proto3"
message Money {
`

	const styleViolationsProto = `syntax = "proto3";
package acme.pay.v1;
enum currency {
  CURRENCY_USD = 1;
  CURRENCY_UNSPECIFIED = 0;
  eur = 2;
}
message money {
  int64 TotalUnits = 1;
  oneof PaymentChoice {
    string CardId = 2;
  }
}
service payService {
  rpc get_balance(money) returns (money);
}
`

	const validProto2 = `syntax = "proto2";
package acme.legacy.v1;
enum Status {
  STATUS_ACTIVE = 1;
}
message LegacyOrder {
  optional Status status = 1;
}
`

	const hexZeroProto3 = `syntax = "proto3";
package acme.hex.v1;
enum Mode {
  MODE_UNSPECIFIED = 0x0;
  MODE_FAST = 1;
}
`

	tests := []struct {
		name            string
		files           map[string]string
		args            []string
		wantErr         string
		wantSubstrs     []string
		wantRules       []string
		wantAbsentRules []string
		wantClean       bool
		wantJSON        bool
	}{
		{
			name:      "valid schema passes",
			files:     map[string]string{"api/pay.proto": validProto},
			args:      []string{"lint"},
			wantClean: true,
		},
		{
			name:      "valid proto2 enum without zero value passes",
			files:     map[string]string{"api/legacy.proto": validProto2},
			args:      []string{"lint"},
			wantClean: true,
		},
		{
			name:      "valid proto3 enum with hex 0x0 first value passes",
			files:     map[string]string{"api/hex.proto": hexZeroProto3},
			args:      []string{"lint"},
			wantClean: true,
		},
		{
			name:    "syntax error reported",
			files:   map[string]string{"api/broken.proto": brokenProto},
			args:    []string{"lint"},
			wantErr: "lint failed",
			wantSubstrs: []string{
				"SYNTAX",
			},
			wantRules: []string{"SYNTAX"},
		},
		{
			name:    "missing syntax and no zero enum value",
			files:   map[string]string{"api/nosyntax.proto": missingSyntaxProto},
			args:    []string{"lint"},
			wantErr: "lint failed",
			wantSubstrs: []string{
				"missing explicit syntax",
			},
			wantRules: []string{"SYNTAX"},
		},
		{
			name:    "naming violations and non-first proto3 zero enum reported",
			files:   map[string]string{"api/style.proto": styleViolationsProto},
			args:    []string{"lint"},
			wantErr: "lint failed",
			wantSubstrs: []string{
				`oneof "PaymentChoice" should be snake_case`,
			},
			wantRules: []string{
				"ENUM_NAME",
				"ENUM_VALUE_NAME",
				"ENUM_ZERO_VALUE",
				"MESSAGE_NAME",
				"FIELD_NAME",
				"SERVICE_NAME",
				"RPC_NAME",
			},
		},
		{
			name:     "json output via --format for valid schema",
			files:    map[string]string{"api/pay.proto": validProto},
			args:     []string{"lint", "--format", "json"},
			wantJSON: true,
		},
		{
			name:     "json output via -o shorthand and deduplicated overlapping paths",
			files:    map[string]string{"api/v1/pay.proto": validProto},
			args:     []string{"lint", "-o", "json", "api", "api/v1"},
			wantJSON: true,
		},
		{
			name:    "invalid format rejected",
			files:   map[string]string{"api/pay.proto": validProto},
			args:    []string{"lint", "--format", "xml"},
			wantErr: "invalid --format",
		},
		{
			name:    "invalid output flag rejected",
			files:   map[string]string{"api/pay.proto": validProto},
			args:    []string{"lint", "-o", "yaml"},
			wantErr: "invalid --output",
		},
		{
			name:        "file name violation reported at line one",
			files:       map[string]string{"api/v1/OrderService.proto": validProto},
			args:        []string{"lint", "-o", "json"},
			wantErr:     "lint failed: 1 violation(s) found",
			wantRules:   []string{"FILE_NAME"},
			wantSubstrs: []string{"should be lower_snake_case"},
			wantJSON:    true,
		},
		{
			name:            "ignored file name passes with clean table output",
			files:           map[string]string{"api/OrderService.proto": validProto},
			args:            []string{"lint", "--ignore-rule", "FILE_NAME"},
			wantAbsentRules: []string{"FILE_NAME"},
			wantClean:       true,
		},
		{
			name:            "ignored file names preserve file count and empty JSON array",
			files:           map[string]string{"api/OrderService.proto": validProto, "api/PayService.proto": validProto},
			args:            []string{"lint", "--ignore-rule", "file_name", "--format", "json"},
			wantAbsentRules: []string{"FILE_NAME"},
			wantJSON:        true,
		},
		{
			name:  "repeated comma-separated rules normalize case and whitespace and allow duplicates",
			files: map[string]string{"api/OrderService.proto": styleViolationsProto},
			args: []string{"lint", "--ignore-rule", " file_name , enum_name,ENUM_VALUE_NAME ",
				"--ignore-rule", "enum_zero_value,MESSAGE_NAME", "--ignore-rule", "field_NAME,SERVICE_NAME,rpc_name,FILE_NAME"},
			wantAbsentRules: []string{"FILE_NAME", "ENUM_NAME", "ENUM_VALUE_NAME", "ENUM_ZERO_VALUE", "MESSAGE_NAME", "FIELD_NAME", "SERVICE_NAME", "RPC_NAME"},
			wantClean:       true,
		},
		{
			name:            "partial suppression keeps remaining findings and table totals",
			files:           map[string]string{"api/OrderService.proto": missingSyntaxProto},
			args:            []string{"lint", "--ignore-rule", "FILE_NAME"},
			wantErr:         "lint failed: 1 violation(s) found",
			wantRules:       []string{"SYNTAX"},
			wantAbsentRules: []string{"FILE_NAME"},
			wantSubstrs:     []string{"1 issue(s) found across 1 file(s)."},
		},
		{
			name:            "partial suppression keeps remaining JSON findings and exit status",
			files:           map[string]string{"api/OrderService.proto": brokenProto},
			args:            []string{"lint", "--ignore-rule", "FILE_NAME", "-o", "json"},
			wantErr:         "lint failed: 1 violation(s) found",
			wantRules:       []string{"SYNTAX"},
			wantAbsentRules: []string{"FILE_NAME"},
			wantJSON:        true,
		},
		{
			name:            "file name checked even when syntax cannot be parsed",
			files:           map[string]string{"api/OrderService.proto": brokenProto},
			args:            []string{"lint", "--ignore-rule", "syntax", "-o", "json"},
			wantErr:         "lint failed: 1 violation(s) found",
			wantRules:       []string{"FILE_NAME"},
			wantAbsentRules: []string{"SYNTAX"},
			wantJSON:        true,
		},
		{
			name:            "syntax finding can be ignored",
			files:           map[string]string{"api/broken.proto": brokenProto},
			args:            []string{"lint", "--ignore-rule", "SYNTAX", "-o", "json"},
			wantAbsentRules: []string{"SYNTAX"},
			wantJSON:        true,
		},
		{
			name:            "package finding can be ignored",
			files:           map[string]string{"api/pay.proto": strings.Replace(validProto, "package acme.pay.v1;", "package acme.pay.v1; package duplicate;", 1)},
			args:            []string{"lint", "--ignore-rule", "PACKAGE"},
			wantAbsentRules: []string{"PACKAGE"},
			wantClean:       true,
		},
		{
			name:      "ignoring a rule with no findings passes",
			files:     map[string]string{"api/pay.proto": validProto},
			args:      []string{"lint", "--ignore-rule", "FILE_NAME"},
			wantClean: true,
		},
		{
			name:    "explicit path argument",
			files:   map[string]string{"schemas/style.proto": styleViolationsProto},
			args:    []string{"lint", "schemas"},
			wantErr: "lint failed",
			wantRules: []string{
				"MESSAGE_NAME",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			origDir, err := os.Getwd()
			require.NoError(t, err)
			require.NoError(t, os.Chdir(tmpDir))
			defer func() { _ = os.Chdir(origDir) }()

			for name, content := range tc.files {
				require.NoError(t, os.MkdirAll(filepath.Dir(name), 0o755))
				require.NoError(t, os.WriteFile(name, []byte(content), 0o644))
			}

			lintCmd := NewLintCmd(&model.Config{Export: model.Export{Paths: []string{"api"}}})
			var out bytes.Buffer
			lintCmd.SetOut(&out)
			lintCmd.SetErr(&out)
			// drop the leading "lint" token since the command is executed directly
			lintCmd.SetArgs(tc.args[1:])

			err = lintCmd.Execute()
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			got := out.String()
			if tc.wantClean {
				assert.Contains(t, got, "No issues found.")
			}
			for _, sub := range tc.wantSubstrs {
				assert.Contains(t, got, sub)
			}
			for _, rule := range tc.wantRules {
				assert.Contains(t, got, rule)
			}
			for _, rule := range tc.wantAbsentRules {
				assert.NotContains(t, got, rule)
			}
			if tc.wantJSON {
				var parsed LintReport
				require.NoError(t, json.Unmarshal(out.Bytes(), &parsed))
				assert.Equal(t, len(tc.files), parsed.Files)
				assert.NotNil(t, parsed.Findings)
				require.Len(t, parsed.Findings, len(tc.wantRules))
				for i, finding := range parsed.Findings {
					assert.Equal(t, tc.wantRules[i], finding.Rule)
					assert.Contains(t, tc.files, finding.Filename)
					if finding.Rule == "FILE_NAME" {
						assert.Equal(t, 1, finding.Line)
						assert.Equal(t, 1, finding.Column)
					}
				}
			}
		})
	}

	t.Run("invalid ignore rules fail before collecting files", func(t *testing.T) {
		for _, value := range []string{"FILE_NAM", "file-name", "FILE NAME", "", " \t ", ",FILE_NAME", "FILE_NAME,", "FILE_NAME,,SYNTAX", "FILE_NAME,BOGUS"} {
			t.Run(value, func(t *testing.T) {
				lintCmd := NewLintCmd(&model.Config{})
				var out bytes.Buffer
				lintCmd.SetOut(&out)
				lintCmd.SetErr(&out)
				lintCmd.SetArgs([]string{"--ignore-rule", value, filepath.Join(t.TempDir(), "missing.proto")})
				require.ErrorContains(t, lintCmd.Execute(), "invalid --ignore-rule")
				assert.Empty(t, out.String())
			})
		}
	})

	t.Run("file name edge cases", func(t *testing.T) {
		for _, filename := range []string{"order.proto", "api/v1/order_service.proto", "Upper-Dir/order_v2.proto", "v1.proto"} {
			t.Run(filename, func(t *testing.T) {
				assert.Empty(t, lintProtoFile(filename, validProto))
			})
		}
		for _, filename := range []string{"OrderService.proto", "order-service.proto", "order.service.proto", "order__service.proto", "_order.proto", "order_.proto", "1order.proto", ".proto"} {
			t.Run(filename, func(t *testing.T) {
				findings := lintProtoFile(filename, validProto)
				require.Len(t, findings, 1)
				assert.Equal(t, "FILE_NAME", findings[0].Rule)
				assert.Equal(t, filename, findings[0].Filename)
				assert.Equal(t, 1, findings[0].Line)
				assert.Equal(t, 1, findings[0].Column)
			})
		}
	})

	t.Run("runs offline via NewRootCmd with symlinked dirs, proto fallback, and invalid env vars", func(t *testing.T) {
		parentDir := t.TempDir()
		workDir := filepath.Join(parentDir, "service")
		sharedDir := filepath.Join(parentDir, "shared")
		absFile := filepath.Join(parentDir, "abs", "extra.proto")
		symlinkDir := filepath.Join(workDir, "linked-api")
		emptyDir := filepath.Join(workDir, "empty")
		require.NoError(t, os.MkdirAll(workDir, 0o755))
		require.NoError(t, os.MkdirAll(sharedDir, 0o755))
		require.NoError(t, os.MkdirAll(emptyDir, 0o755))
		require.NoError(t, os.MkdirAll(filepath.Dir(absFile), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(sharedDir, "pay.proto"), []byte(validProto), 0o644))
		require.NoError(t, os.WriteFile(absFile, []byte(validProto), 0o644))
		require.NoError(t, os.Symlink(sharedDir, symlinkDir))

		// Write invalid pbuf.yaml in CWD to verify explicit path args bypass CWD config loading
		require.NoError(t, os.WriteFile(filepath.Join(workDir, model.PbufConfigFilename), []byte("invalid: [yaml"), 0o644))

		origDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(workDir))
		defer func() { _ = os.Chdir(origDir) }()
		t.Setenv("PBUF_REGISTRY_ADDR", "localhost:6777")
		t.Setenv("PBUF_REGISTRY_INSECURE", "not-a-bool")

		rootCmd := NewRootCmd()
		var out bytes.Buffer
		rootCmd.SetOut(&out)
		rootCmd.SetErr(&out)
		rootCmd.SetArgs([]string{"lint", "-o", "json", "linked-api", "../shared", absFile})
		require.NoError(t, rootCmd.Execute())

		var parsed LintReport
		require.NoError(t, json.Unmarshal(out.Bytes(), &parsed))
		assert.Equal(t, 2, parsed.Files)
		assert.Empty(t, parsed.Findings)

		// Empty directory must fail instead of passing silently
		emptyCmd := NewRootCmd()
		emptyCmd.SetOut(&bytes.Buffer{})
		emptyCmd.SetErr(&bytes.Buffer{})
		emptyCmd.SetArgs([]string{"lint", "empty"})
		require.ErrorContains(t, emptyCmd.Execute(), "no .proto files found")

		// Conflicting -o and --format flags must fail
		conflictCmd := NewRootCmd()
		conflictCmd.SetOut(&bytes.Buffer{})
		conflictCmd.SetErr(&bytes.Buffer{})
		conflictCmd.SetArgs([]string{"lint", "-o", "json", "--format", "table", absFile})
		require.ErrorContains(t, conflictCmd.Execute(), "conflicting --output")

		// Without pbuf.yaml, default resolution falls back to proto/ and ignores PBUF_REGISTRY_INSECURE
		require.NoError(t, os.Remove(filepath.Join(workDir, model.PbufConfigFilename)))
		require.NoError(t, os.MkdirAll(filepath.Join(workDir, "proto"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(workDir, "proto", "bad.proto"), []byte(`syntax = "proto3";
package acme.v1;
package acme.v2;
message Outer {
  map<string, string> BadMap = 1;
  message nestedMsg {
    enum nestedEnum {
      VAL_ONE = 1;
    }
  }
}
`), 0o644))

		fallbackCmd := NewRootCmd()
		var failOut bytes.Buffer
		fallbackCmd.SetOut(&failOut)
		fallbackCmd.SetErr(&bytes.Buffer{})
		fallbackCmd.SetArgs([]string{"lint", "-o", "json"})
		require.ErrorContains(t, fallbackCmd.Execute(), "lint failed")

		var failReport LintReport
		require.NoError(t, json.Unmarshal(failOut.Bytes(), &failReport))
		assert.Equal(t, 1, failReport.Files)
		require.NotEmpty(t, failReport.Findings)
		var gotRules []string
		for _, f := range failReport.Findings {
			assert.Greater(t, f.Line, 0)
			gotRules = append(gotRules, f.Rule)
		}
		assert.Contains(t, gotRules, "PACKAGE")
		assert.Contains(t, gotRules, "FIELD_NAME")
		assert.Contains(t, gotRules, "MESSAGE_NAME")
		assert.Contains(t, gotRules, "ENUM_NAME")
		assert.Contains(t, gotRules, "ENUM_ZERO_VALUE")
	})
}

func TestShellCompletionCommand(t *testing.T) {
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(t.TempDir()))
	t.Setenv("PBUF_REGISTRY_ADDR", "")

	shells := []struct {
		shell      string
		wantMarker string
	}{
		{shell: "bash", wantMarker: "__start_pbuf"},
		{shell: "zsh", wantMarker: "#compdef pbuf"},
		{shell: "fish", wantMarker: "complete -c pbuf"},
		{shell: "powershell", wantMarker: "Register-ArgumentCompleter"},
	}

	for _, tc := range shells {
		t.Run(tc.shell, func(t *testing.T) {
			rootCmd := NewRootCmd()
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			rootCmd.SetArgs([]string{"completion", tc.shell})

			require.NoError(t, rootCmd.Execute())
			assert.Contains(t, out.String(), tc.wantMarker)
		})
	}

	t.Run("dynamic __complete suggests all subcommands and flags outside pbuf.yaml dir", func(t *testing.T) {
		rootCmd := NewRootCmd()
		var out, errBuf bytes.Buffer
		rootCmd.SetOut(&out)
		rootCmd.SetErr(&errBuf)
		rootCmd.SetArgs([]string{"__complete", ""})
		require.NoError(t, rootCmd.Execute())

		got := out.String()
		for _, sub := range []string{"init", "auth", "login", "vendor", "modules", "drift", "metadata", "users", "mcp", "completion"} {
			assert.Contains(t, got, sub)
		}

		rootFlagsCmd := NewRootCmd()
		out.Reset()
		errBuf.Reset()
		rootFlagsCmd.SetOut(&out)
		rootFlagsCmd.SetErr(&errBuf)
		rootFlagsCmd.SetArgs([]string{"__complete", "modules", "list", "--"})
		require.NoError(t, rootFlagsCmd.Execute())
		assert.Contains(t, out.String(), "--all")
		assert.Contains(t, out.String(), "--page-size")
		assert.Contains(t, out.String(), "--output")
	})

	t.Run("unguarded leaf commands return clear error when unconfigured", func(t *testing.T) {
		groupCmd := NewRootCmd()
		var out bytes.Buffer
		groupCmd.SetOut(&out)
		groupCmd.SetErr(&out)
		groupCmd.SetArgs([]string{"modules"})
		require.NoError(t, groupCmd.Execute())
		assert.Contains(t, out.String(), "Available Commands")

		leafCmd := NewRootCmd()
		leafCmd.SetOut(&out)
		leafCmd.SetErr(&out)
		leafCmd.SetArgs([]string{"modules", "list"})
		require.ErrorContains(t, leafCmd.Execute(), "no registry configured")

		vendorCmd := NewRootCmd()
		vendorCmd.SetOut(&out)
		vendorCmd.SetErr(&out)
		vendorCmd.SetArgs([]string{"vendor"})
		require.ErrorContains(t, vendorCmd.Execute(), "no pbuf.yaml file found")
	})

	t.Run("leaf help succeeds when unconfigured", func(t *testing.T) {
		for _, args := range [][]string{
			{"modules", "list", "--help"},
			{"help", "modules", "list"},
			{"vendor", "--help"},
		} {
			rootCmd := NewRootCmd()
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			rootCmd.SetArgs(args)
			require.NoError(t, rootCmd.Execute(), "args: %v", args)
			assert.Contains(t, out.String(), "Usage:")
		}
	})

	t.Run("invalid registry addr or malformed pbuf.yaml does not break completion or help", func(t *testing.T) {
		t.Setenv("PBUF_REGISTRY_ADDR", "%zz:6777")
		t.Setenv("PBUF_REGISTRY_INSECURE", "true")

		for _, args := range [][]string{
			{"completion", "zsh"},
			{"__complete", ""},
			{"modules", "list", "--help"},
		} {
			rootCmd := NewRootCmd()
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			rootCmd.SetArgs(args)
			require.NoError(t, rootCmd.Execute(), "args: %v", args)
		}

		badAddrCmd := NewRootCmd()
		var out bytes.Buffer
		badAddrCmd.SetOut(&out)
		badAddrCmd.SetErr(&out)
		badAddrCmd.SetArgs([]string{"modules", "list"})
		require.ErrorContains(t, badAddrCmd.Execute(), "failed to create grpc client")

		t.Setenv("PBUF_REGISTRY_ADDR", "")
		require.NoError(t, os.WriteFile(model.PbufConfigFilename, []byte(":::invalid:yaml:[\n"), 0o644))
		t.Cleanup(func() { _ = os.Remove(model.PbufConfigFilename) })

		compCmd := NewRootCmd()
		out.Reset()
		compCmd.SetOut(&out)
		compCmd.SetErr(&out)
		compCmd.SetArgs([]string{"__complete", "modules", ""})
		require.NoError(t, compCmd.Execute())
		assert.Contains(t, out.String(), "list")

		badConfigCmd := NewRootCmd()
		out.Reset()
		badConfigCmd.SetOut(&out)
		badConfigCmd.SetErr(&out)
		badConfigCmd.SetArgs([]string{"modules", "list"})
		require.ErrorContains(t, badConfigCmd.Execute(), "failed to parse pbuf.yaml")
	})

	t.Run("PBUF_REGISTRY_INSECURE overrides pbuf.yaml in both directions", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		srv := grpc.NewServer()
		v1.RegisterRegistryServer(srv, v1.UnimplementedRegistryServer{})
		go func() { _ = srv.Serve(ln) }()
		t.Cleanup(srv.Stop)

		origDir, err := os.Getwd()
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.Chdir(origDir) })
		require.NoError(t, os.Chdir(t.TempDir()))
		t.Setenv("PBUF_REGISTRY_ADDR", "")

		require.NoError(t, os.WriteFile(model.PbufConfigFilename, []byte("version: v1\nregistry:\n  addr: "+ln.Addr().String()+"\n"), 0o644))
		t.Setenv("PBUF_REGISTRY_INSECURE", "true")
		insecureCmd := NewRootCmd()
		insecureCmd.SetOut(&bytes.Buffer{})
		insecureCmd.SetErr(&bytes.Buffer{})
		insecureCmd.SetArgs([]string{"modules", "list"})
		require.ErrorContains(t, insecureCmd.Execute(), "Unimplemented")

		require.NoError(t, os.WriteFile(model.PbufConfigFilename, []byte("version: v1\nregistry:\n  addr: "+ln.Addr().String()+"\n  insecure: true\n"), 0o644))
		t.Setenv("PBUF_REGISTRY_INSECURE", "false")
		secureCmd := NewRootCmd()
		secureCmd.SetOut(&bytes.Buffer{})
		secureCmd.SetErr(&bytes.Buffer{})
		secureCmd.SetArgs([]string{"modules", "list"})
		err = secureCmd.Execute()
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "Unimplemented")

		t.Setenv("PBUF_REGISTRY_INSECURE", "not-a-bool")
		invalidCmd := NewRootCmd()
		invalidCmd.SetOut(&bytes.Buffer{})
		invalidCmd.SetErr(&bytes.Buffer{})
		invalidCmd.SetArgs([]string{"modules", "list"})
		require.ErrorContains(t, invalidCmd.Execute(), "invalid PBUF_REGISTRY_INSECURE value")
	})
}
