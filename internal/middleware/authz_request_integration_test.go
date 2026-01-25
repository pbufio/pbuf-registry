package middleware

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type registryTestServer struct {
	v1.UnimplementedRegistryServer
}

type metadataTestServer struct {
	v1.UnimplementedMetadataServiceServer
}

func (s *metadataTestServer) GetMetadata(_ context.Context, _ *v1.GetMetadataRequest) (*v1.GetMetadataResponse, error) {
	return &v1.GetMetadataResponse{}, nil
}

func (s *registryTestServer) RegisterModule(_ context.Context, request *v1.RegisterModuleRequest) (*v1.Module, error) {
	return &v1.Module{Name: request.GetName()}, nil
}

func (s *registryTestServer) GetModule(_ context.Context, request *v1.GetModuleRequest) (*v1.Module, error) {
	return &v1.Module{Name: request.GetName()}, nil
}

func (s *registryTestServer) DeleteModule(_ context.Context, request *v1.DeleteModuleRequest) (*v1.DeleteModuleResponse, error) {
	return &v1.DeleteModuleResponse{Name: request.GetName()}, nil
}

func startAuthzGRPCServer(t *testing.T, adminToken string) (v1.RegistryClient, v1.MetadataServiceClient, func()) {
	t.Helper()

	buffer := 101024 * 1024
	lis := bufconn.Listen(buffer)

	auth := NewACLAuth(adminToken, integrationSuite.userRepository, log.DefaultLogger)
	srv := kratosgrpc.NewServer(
		kratosgrpc.Listener(lis),
		kratosgrpc.Middleware(
			auth.NewAuthMiddleware(),
			NewAuthorizationMiddleware(integrationSuite.aclRepository, log.DefaultLogger),
		),
	)
	v1.RegisterRegistryServer(srv, &registryTestServer{})
	v1.RegisterMetadataServiceServer(srv, &metadataTestServer{})

	go func() {
		_ = srv.Serve(lis)
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	// grpc.NewClient performs no I/O; connect eagerly so tests fail fast.
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer connectCancel()
	conn.Connect()
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			break
		}
		if !conn.WaitForStateChange(connectCtx, state) {
			require.FailNowf(t, "timeout", "timeout waiting for gRPC client connection; last state: %v", state)
		}
	}

	closer := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
		_ = lis.Close()
		_ = conn.Close()
	}

	return v1.NewRegistryClient(conn), v1.NewMetadataServiceClient(conn), closer
}

func ctxWithAuthToken(ctx context.Context, token string) context.Context {
	if token == "" {
		return ctx
	}
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", token))
}

func requirePermissionDenied(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func requireUnauthenticated(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func Test_authorization_GRPC_RealRequests_PermissionsPerRequest(t *testing.T) {
	adminToken := "admin-secret-token-authz"
	registryClient, metadataClient, closeServer := startAuthzGRPCServer(t, adminToken)
	defer closeServer()

	t.Run("missing token is unauthenticated", func(t *testing.T) {
		_, err := registryClient.GetModule(context.Background(), &v1.GetModuleRequest{Name: "mod-no-token"})
		requireUnauthenticated(t, err)

		_, err = metadataClient.GetMetadata(context.Background(), &v1.GetMetadataRequest{Name: "mod-no-token", Tag: "v1.0.0"})
		requireUnauthenticated(t, err)
	})

	t.Run("deny by default (no ACL entries)", func(t *testing.T) {
		plainToken := "pbuf_user_authz_deny_default_" + t.Name()
		_ = createTestUser(t, "authz-deny-default-"+t.Name(), plainToken)

		ctx := ctxWithAuthToken(context.Background(), plainToken)
		_, err := registryClient.GetModule(ctx, &v1.GetModuleRequest{Name: "mod-a"})
		requirePermissionDenied(t, err)

		_, err = registryClient.RegisterModule(ctx, &v1.RegisterModuleRequest{Name: "mod-a"})
		requirePermissionDenied(t, err)

		_, err = metadataClient.GetMetadata(ctx, &v1.GetMetadataRequest{Name: "mod-a", Tag: "v1.0.0"})
		requirePermissionDenied(t, err)
	})

	t.Run("wildcard read allows read ops, denies write ops", func(t *testing.T) {
		plainToken := "pbuf_user_authz_wildcard_read_" + t.Name()
		user := createTestUser(t, "authz-wildcard-read-"+t.Name(), plainToken)

		err := integrationSuite.aclRepository.GrantPermission(context.Background(), &model.ACLEntry{
			UserID:     user.ID,
			ModuleName: "*",
			Permission: model.PermissionRead,
		})
		require.NoError(t, err)

		ctx := ctxWithAuthToken(context.Background(), plainToken)
		resp, err := registryClient.GetModule(ctx, &v1.GetModuleRequest{Name: "mod-a"})
		require.NoError(t, err)
		assert.Equal(t, "mod-a", resp.Name)

		_, err = metadataClient.GetMetadata(ctx, &v1.GetMetadataRequest{Name: "mod-a", Tag: "v1.0.0"})
		require.NoError(t, err)

		_, err = registryClient.RegisterModule(ctx, &v1.RegisterModuleRequest{Name: "mod-a"})
		requirePermissionDenied(t, err)
	})

	t.Run("module-specific read allows metadata only for that module", func(t *testing.T) {
		plainToken := "pbuf_user_authz_module_read_metadata_" + t.Name()
		user := createTestUser(t, "authz-module-read-metadata-"+t.Name(), plainToken)

		err := integrationSuite.aclRepository.GrantPermission(context.Background(), &model.ACLEntry{
			UserID:     user.ID,
			ModuleName: "mod-a",
			Permission: model.PermissionRead,
		})
		require.NoError(t, err)

		ctx := ctxWithAuthToken(context.Background(), plainToken)
		_, err = metadataClient.GetMetadata(ctx, &v1.GetMetadataRequest{Name: "mod-a", Tag: "v1.0.0"})
		require.NoError(t, err)

		_, err = metadataClient.GetMetadata(ctx, &v1.GetMetadataRequest{Name: "mod-b", Tag: "v1.0.0"})
		requirePermissionDenied(t, err)
	})

	t.Run("module-specific write allows write only for that module", func(t *testing.T) {
		plainToken := "pbuf_user_authz_module_write_" + t.Name()
		user := createTestUser(t, "authz-module-write-"+t.Name(), plainToken)

		err := integrationSuite.aclRepository.GrantPermission(context.Background(), &model.ACLEntry{
			UserID:     user.ID,
			ModuleName: "mod-a",
			Permission: model.PermissionWrite,
		})
		require.NoError(t, err)

		ctx := ctxWithAuthToken(context.Background(), plainToken)
		resp, err := registryClient.RegisterModule(ctx, &v1.RegisterModuleRequest{Name: "mod-a"})
		require.NoError(t, err)
		assert.Equal(t, "mod-a", resp.Name)

		_, err = registryClient.RegisterModule(ctx, &v1.RegisterModuleRequest{Name: "mod-b"})
		requirePermissionDenied(t, err)
	})

	t.Run("specific permission overrides wildcard (checked first)", func(t *testing.T) {
		plainToken := "pbuf_user_authz_override_" + t.Name()
		user := createTestUser(t, "authz-override-"+t.Name(), plainToken)

		err := integrationSuite.aclRepository.GrantPermission(context.Background(), &model.ACLEntry{
			UserID:     user.ID,
			ModuleName: "*",
			Permission: model.PermissionWrite,
		})
		require.NoError(t, err)

		err = integrationSuite.aclRepository.GrantPermission(context.Background(), &model.ACLEntry{
			UserID:     user.ID,
			ModuleName: "mod-a",
			Permission: model.PermissionRead,
		})
		require.NoError(t, err)

		ctx := ctxWithAuthToken(context.Background(), plainToken)

		// mod-a: exact read entry is evaluated first -> write op is denied.
		_, err = registryClient.RegisterModule(ctx, &v1.RegisterModuleRequest{Name: "mod-a"})
		requirePermissionDenied(t, err)

		// mod-b: no exact entry -> wildcard write allows.
		resp, err := registryClient.RegisterModule(ctx, &v1.RegisterModuleRequest{Name: "mod-b"})
		require.NoError(t, err)
		assert.Equal(t, "mod-b", resp.Name)
	})

	t.Run("admin token bypass allows admin operations", func(t *testing.T) {
		ctx := ctxWithAuthToken(context.Background(), "Bearer "+adminToken)
		_, err := registryClient.DeleteModule(ctx, &v1.DeleteModuleRequest{Name: "mod-admin"})
		require.NoError(t, err)
	})
}
