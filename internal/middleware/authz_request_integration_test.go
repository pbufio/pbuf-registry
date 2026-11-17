package middleware

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/google/uuid"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/mocks"
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

type driftTestServer struct {
	v1.UnimplementedDriftServiceServer
}

func (s *driftTestServer) ListDriftEvents(_ context.Context, _ *v1.ListDriftEventsRequest) (*v1.ListDriftEventsResponse, error) {
	return &v1.ListDriftEventsResponse{}, nil
}

func (s *driftTestServer) GetModuleDriftEvents(_ context.Context, req *v1.GetModuleDriftEventsRequest) (*v1.GetModuleDriftEventsResponse, error) {
	return &v1.GetModuleDriftEventsResponse{}, nil
}

func (s *driftTestServer) AcknowledgeDriftEvent(_ context.Context, req *v1.AcknowledgeDriftEventRequest) (*v1.AcknowledgeDriftEventResponse, error) {
	return &v1.AcknowledgeDriftEventResponse{
		Event: &v1.DriftEvent{
			Id:           req.GetEventId(),
			Acknowledged: true,
		},
	}, nil
}

func (s *registryTestServer) ListModules(_ context.Context, _ *v1.ListModulesRequest) (*v1.ListModulesResponse, error) {
	return &v1.ListModulesResponse{}, nil
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

func startAuthzGRPCServer(t *testing.T, adminToken string) (v1.RegistryClient, v1.MetadataServiceClient, v1.DriftServiceClient, func()) {
	t.Helper()

	buffer := 101024 * 1024
	lis := bufconn.Listen(buffer)

	auth := NewACLAuth(adminToken, integrationSuite.userRepository, log.DefaultLogger)
	var driftRepo data.DriftRepository
	if integrationSuite.pool != nil {
		driftRepo = data.NewDriftRepository(integrationSuite.pool, log.DefaultLogger)
	}
	srv := kratosgrpc.NewServer(
		kratosgrpc.Listener(lis),
		kratosgrpc.Middleware(
			auth.NewAuthMiddleware(),
			NewAuthorizationMiddleware(integrationSuite.aclRepository, log.DefaultLogger, driftRepo),
		),
	)
	v1.RegisterRegistryServer(srv, &registryTestServer{})
	v1.RegisterMetadataServiceServer(srv, &metadataTestServer{})
	v1.RegisterDriftServiceServer(srv, &driftTestServer{})

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

	return v1.NewRegistryClient(conn), v1.NewMetadataServiceClient(conn), v1.NewDriftServiceClient(conn), closer
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
	registryClient, metadataClient, driftClient, closeServer := startAuthzGRPCServer(t, adminToken)
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

	// DriftService tests
	t.Run("drift: missing token is unauthenticated", func(t *testing.T) {
		_, err := driftClient.ListDriftEvents(context.Background(), &v1.ListDriftEventsRequest{})
		requireUnauthenticated(t, err)

		_, err = driftClient.GetModuleDriftEvents(context.Background(), &v1.GetModuleDriftEventsRequest{ModuleName: "test.module"})
		requireUnauthenticated(t, err)

		_, err = driftClient.AcknowledgeDriftEvent(context.Background(), &v1.AcknowledgeDriftEventRequest{EventId: "event-id"})
		requireUnauthenticated(t, err)
	})

	t.Run("drift: deny by default (no ACL entries)", func(t *testing.T) {
		plainToken := "pbuf_user_authz_drift_deny_" + t.Name()
		_ = createTestUser(t, "authz-drift-deny-"+t.Name(), plainToken)

		ctx := ctxWithAuthToken(context.Background(), plainToken)
		_, err := driftClient.ListDriftEvents(ctx, &v1.ListDriftEventsRequest{})
		requirePermissionDenied(t, err)

		_, err = driftClient.GetModuleDriftEvents(ctx, &v1.GetModuleDriftEventsRequest{ModuleName: "test.module"})
		requirePermissionDenied(t, err)

		_, err = driftClient.AcknowledgeDriftEvent(ctx, &v1.AcknowledgeDriftEventRequest{EventId: "event-id"})
		requirePermissionDenied(t, err)
	})

	t.Run("drift: wildcard read allows list and get, denies acknowledge", func(t *testing.T) {
		plainToken := "pbuf_user_authz_drift_read_" + t.Name()
		user := createTestUser(t, "authz-drift-read-"+t.Name(), plainToken)

		err := integrationSuite.aclRepository.GrantPermission(context.Background(), &model.ACLEntry{
			UserID:     user.ID,
			ModuleName: "*",
			Permission: model.PermissionRead,
		})
		require.NoError(t, err)

		ctx := ctxWithAuthToken(context.Background(), plainToken)
		_, err = driftClient.ListDriftEvents(ctx, &v1.ListDriftEventsRequest{})
		require.NoError(t, err)

		_, err = driftClient.GetModuleDriftEvents(ctx, &v1.GetModuleDriftEventsRequest{ModuleName: "test.module"})
		require.NoError(t, err)

		// Acknowledge requires write permission
		_, err = driftClient.AcknowledgeDriftEvent(ctx, &v1.AcknowledgeDriftEventRequest{EventId: "event-id"})
		requirePermissionDenied(t, err)
	})

	t.Run("drift: wildcard write allows acknowledge", func(t *testing.T) {
		plainToken := "pbuf_user_authz_drift_write_" + t.Name()
		user := createTestUser(t, "authz-drift-write-"+t.Name(), plainToken)

		err := integrationSuite.aclRepository.GrantPermission(context.Background(), &model.ACLEntry{
			UserID:     user.ID,
			ModuleName: "*",
			Permission: model.PermissionWrite,
		})
		require.NoError(t, err)

		ctx := ctxWithAuthToken(context.Background(), plainToken)
		resp, err := driftClient.AcknowledgeDriftEvent(ctx, &v1.AcknowledgeDriftEventRequest{EventId: "event-123", AcknowledgedBy: "test-user"})
		require.NoError(t, err)
		assert.Equal(t, "event-123", resp.GetEvent().GetId())
		assert.True(t, resp.GetEvent().GetAcknowledged())
	})

	t.Run("drift: module-specific read allows GetModuleDriftEvents for that module only", func(t *testing.T) {
		plainToken := "pbuf_user_authz_drift_module_read_" + t.Name()
		user := createTestUser(t, "authz-drift-module-read-"+t.Name(), plainToken)

		err := integrationSuite.aclRepository.GrantPermission(context.Background(), &model.ACLEntry{
			UserID:     user.ID,
			ModuleName: "test.module",
			Permission: model.PermissionRead,
		})
		require.NoError(t, err)

		ctx := ctxWithAuthToken(context.Background(), plainToken)
		_, err = driftClient.GetModuleDriftEvents(ctx, &v1.GetModuleDriftEventsRequest{ModuleName: "test.module"})
		require.NoError(t, err)

		_, err = driftClient.GetModuleDriftEvents(ctx, &v1.GetModuleDriftEventsRequest{ModuleName: "other.module"})
		requirePermissionDenied(t, err)

		// Global list still requires wildcard read
		_, err = driftClient.ListDriftEvents(ctx, &v1.ListDriftEventsRequest{})
		requirePermissionDenied(t, err)
	})

	t.Run("drift: admin token bypass allows all drift operations", func(t *testing.T) {
		ctx := ctxWithAuthToken(context.Background(), "Bearer "+adminToken)

		_, err := driftClient.ListDriftEvents(ctx, &v1.ListDriftEventsRequest{})
		require.NoError(t, err)

		_, err = driftClient.GetModuleDriftEvents(ctx, &v1.GetModuleDriftEventsRequest{ModuleName: "test.module"})
		require.NoError(t, err)

		_, err = driftClient.AcknowledgeDriftEvent(ctx, &v1.AcknowledgeDriftEventRequest{EventId: "event-id"})
		require.NoError(t, err)
	})

	t.Run("module-scoped ListModules allowed at authz layer and AcknowledgeDriftEvent checks event module", func(t *testing.T) {
		plainToken := "pbuf_user_authz_mod_scoped_" + t.Name()
		user := createTestUser(t, "authz-mod-scoped-"+t.Name(), plainToken)

		bg := context.Background()
		regRepo := data.NewRegistryRepository(integrationSuite.pool, log.DefaultLogger)
		driftRepo := data.NewDriftRepository(integrationSuite.pool, log.DefaultLogger)

		modA := "mod-ack-a-" + t.Name()
		modB := "mod-ack-b-" + t.Name()
		for _, m := range []string{modA, modB} {
			require.NoError(t, regRepo.RegisterModule(bg, m))
			_, err := regRepo.PushModule(bg, m, "v1.0.0", []*v1.ProtoFile{
				{Filename: "svc.proto", Content: `syntax = "proto3"; message Svc {}`},
			})
			require.NoError(t, err)
		}
		tagIDA, err := regRepo.GetModuleTagId(bg, modA, "v1.0.0")
		require.NoError(t, err)
		modIDA, _, err := driftRepo.GetTagInfo(bg, tagIDA)
		require.NoError(t, err)
		savedA, err := driftRepo.SaveDriftEvents(bg, []model.DriftEvent{{
			ModuleID: modIDA, TagID: tagIDA, Filename: "svc.proto",
			EventType: model.DriftEventTypeAdded, CurrentHash: "h1", Severity: model.DriftSeverityInfo, DetectedAt: time.Now(),
		}})
		require.NoError(t, err)
		require.Len(t, savedA, 1)

		tagIDB, err := regRepo.GetModuleTagId(bg, modB, "v1.0.0")
		require.NoError(t, err)
		modIDB, _, err := driftRepo.GetTagInfo(bg, tagIDB)
		require.NoError(t, err)
		savedB, err := driftRepo.SaveDriftEvents(bg, []model.DriftEvent{{
			ModuleID: modIDB, TagID: tagIDB, Filename: "svc.proto",
			EventType: model.DriftEventTypeAdded, CurrentHash: "h2", Severity: model.DriftSeverityInfo, DetectedAt: time.Now(),
		}})
		require.NoError(t, err)
		require.Len(t, savedB, 1)

		require.NoError(t, integrationSuite.aclRepository.GrantPermission(bg, &model.ACLEntry{
			UserID:     user.ID,
			ModuleName: modA,
			Permission: model.PermissionWrite,
		}))

		ctx := ctxWithAuthToken(bg, plainToken)
		_, err = registryClient.ListModules(ctx, &v1.ListModulesRequest{PageSize: 10})
		require.NoError(t, err)

		ackResp, err := driftClient.AcknowledgeDriftEvent(ctx, &v1.AcknowledgeDriftEventRequest{EventId: savedA[0].ID})
		require.NoError(t, err)
		assert.Equal(t, savedA[0].ID, ackResp.GetEvent().GetId())

		_, err = driftClient.AcknowledgeDriftEvent(ctx, &v1.AcknowledgeDriftEventRequest{EventId: savedB[0].ID})
		requirePermissionDenied(t, err)
	})
}

func TestAuthorizationMiddleware_Unit_ListModulesAndAcknowledgeScope(t *testing.T) {
	userID := uuid.New()
	user := &model.User{ID: userID, Name: "alice", IsActive: true, Type: model.UserTypeUser}

	aclRepo := &mocks.ACLRepository{}
	driftRepo := &mocks.DriftRepository{}
	a := &authzMiddleware{
		aclRepo:   aclRepo,
		driftRepo: driftRepo,
		logger:    log.NewHelper(log.DefaultLogger),
	}

	perm, mod := getRequiredPermission("/pbufregistry.v1.Registry/ListModules", &v1.ListModulesRequest{})
	assert.Empty(t, perm)
	assert.Empty(t, mod)

	eventAllowed := uuid.New().String()
	eventDenied := uuid.New().String()
	eventMissing := uuid.New().String()

	driftRepo.On("GetDriftEventModuleName", context.Background(), eventAllowed).Return("acme/orders", nil).Once()
	aclRepo.On("CheckPermission", context.Background(), userID, "acme/orders", model.PermissionWrite).Return(true, nil).Once()
	allowed, err := a.authorizeAcknowledgeDriftEvent(context.Background(), user, &v1.AcknowledgeDriftEventRequest{EventId: eventAllowed})
	require.NoError(t, err)
	assert.True(t, allowed)

	driftRepo.On("GetDriftEventModuleName", context.Background(), eventDenied).Return("acme/payments", nil).Once()
	aclRepo.On("CheckPermission", context.Background(), userID, "acme/payments", model.PermissionWrite).Return(false, nil).Once()
	allowed, err = a.authorizeAcknowledgeDriftEvent(context.Background(), user, &v1.AcknowledgeDriftEventRequest{EventId: eventDenied})
	require.NoError(t, err)
	assert.False(t, allowed)

	driftRepo.On("GetDriftEventModuleName", context.Background(), eventMissing).Return("", data.ErrDriftEventNotFound).Once()
	aclRepo.On("ListUserPermissions", context.Background(), userID).Return([]*model.ACLEntry{
		{UserID: userID, ModuleName: "acme/orders", Permission: model.PermissionRead},
	}, nil).Once()
	allowed, err = a.authorizeAcknowledgeDriftEvent(context.Background(), user, &v1.AcknowledgeDriftEventRequest{EventId: eventMissing})
	require.NoError(t, err)
	assert.False(t, allowed)

	aclRepo.AssertExpectations(t)
	driftRepo.AssertExpectations(t)
}
