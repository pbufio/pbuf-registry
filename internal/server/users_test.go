package server

import (
	"context"
	"errors"
	"io"
	stdlog "log"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/mocks"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func setupUsersClient(t *testing.T, userRepo data.UserRepository, aclRepo data.ACLRepository) (v1.UserServiceClient, func()) {
	t.Helper()

	buffer := 101024 * 1024
	lis := bufconn.Listen(buffer)

	grpcServer := grpc.NewServer()
	logger := log.NewStdLogger(io.Discard)
	v1.RegisterUserServiceServer(grpcServer, NewUsersServer(userRepo, aclRepo, logger))
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			stdlog.Printf("error serving server: %v", err)
		}
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("error connecting to server: %v", err)
	}

	closer := func() {
		_ = conn.Close()
		_ = lis.Close()
		grpcServer.Stop()
	}

	return v1.NewUserServiceClient(conn), closer
}

func requireStatusCode(t *testing.T, err error, code codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", code)
	}
	if status.Code(err) != code {
		t.Fatalf("expected status code %s, got %s (err=%v)", code, status.Code(err), err)
	}
}

func TestUsersServer_NilRequest_ReturnsInvalidRequest(t *testing.T) {
	s := &UsersServer{}
	ctx := context.Background()

	t.Run("CreateUser", func(t *testing.T) {
		_, err := s.CreateUser(ctx, nil)
		if err != ErrInvalidRequest {
			t.Fatalf("expected ErrInvalidRequest, got %v", err)
		}
	})
	t.Run("ListUsers", func(t *testing.T) {
		_, err := s.ListUsers(ctx, nil)
		if err != ErrInvalidRequest {
			t.Fatalf("expected ErrInvalidRequest, got %v", err)
		}
	})
	t.Run("GetUser", func(t *testing.T) {
		_, err := s.GetUser(ctx, nil)
		if err != ErrInvalidRequest {
			t.Fatalf("expected ErrInvalidRequest, got %v", err)
		}
	})
	t.Run("UpdateUser", func(t *testing.T) {
		_, err := s.UpdateUser(ctx, nil)
		if err != ErrInvalidRequest {
			t.Fatalf("expected ErrInvalidRequest, got %v", err)
		}
	})
	t.Run("DeleteUser", func(t *testing.T) {
		_, err := s.DeleteUser(ctx, nil)
		if err != ErrInvalidRequest {
			t.Fatalf("expected ErrInvalidRequest, got %v", err)
		}
	})
	t.Run("RegenerateToken", func(t *testing.T) {
		_, err := s.RegenerateToken(ctx, nil)
		if err != ErrInvalidRequest {
			t.Fatalf("expected ErrInvalidRequest, got %v", err)
		}
	})
	t.Run("GrantPermission", func(t *testing.T) {
		_, err := s.GrantPermission(ctx, nil)
		if err != ErrInvalidRequest {
			t.Fatalf("expected ErrInvalidRequest, got %v", err)
		}
	})
	t.Run("RevokePermission", func(t *testing.T) {
		_, err := s.RevokePermission(ctx, nil)
		if err != ErrInvalidRequest {
			t.Fatalf("expected ErrInvalidRequest, got %v", err)
		}
	})
	t.Run("ListUserPermissions", func(t *testing.T) {
		_, err := s.ListUserPermissions(ctx, nil)
		if err != ErrInvalidRequest {
			t.Fatalf("expected ErrInvalidRequest, got %v", err)
		}
	})
}

func TestUsersServer_CreateUser_GRPC(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		createdAt := time.Now().UTC().Truncate(time.Second)
		updatedAt := createdAt.Add(time.Second)
		id := uuid.New()

		userRepo.On(
			"CreateUser",
			mock.Anything,
			mock.MatchedBy(func(u *model.User) bool {
				return u != nil &&
					u.Name == "alice" &&
					u.Type == model.UserTypeUser &&
					u.IsActive &&
					strings.HasPrefix(u.Token, "pbuf_user_")
			}),
		).Run(func(args mock.Arguments) {
			u := args.Get(1).(*model.User)
			u.ID = id
			u.CreatedAt = createdAt
			u.UpdatedAt = updatedAt
		}).Return(nil).Once()

		resp, err := client.CreateUser(ctx, &v1.CreateUserRequest{Name: "alice", Type: v1.UserType_USER_TYPE_USER})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetUser().GetId() != id.String() {
			t.Fatalf("expected user id %s, got %s", id, resp.GetUser().GetId())
		}
		if resp.GetUser().GetName() != "alice" {
			t.Fatalf("expected user name alice, got %s", resp.GetUser().GetName())
		}
		if resp.GetToken() == "" {
			t.Fatalf("expected token to be set")
		}
		if !strings.HasPrefix(resp.GetToken(), "pbuf_user_") {
			t.Fatalf("expected token prefix pbuf_user_, got %s", resp.GetToken())
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		_, err := client.CreateUser(ctx, &v1.CreateUserRequest{Name: "n", Type: v1.UserType_USER_TYPE_UNSPECIFIED})
		requireStatusCode(t, err, codes.InvalidArgument)
	})

	t.Run("empty name", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		_, err := client.CreateUser(ctx, &v1.CreateUserRequest{Name: "", Type: v1.UserType_USER_TYPE_USER})
		requireStatusCode(t, err, codes.InvalidArgument)
	})

	t.Run("repo error", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		userRepo.On("CreateUser", mock.Anything, mock.Anything).Return(errors.New("db down")).Once()

		_, err := client.CreateUser(ctx, &v1.CreateUserRequest{Name: "alice", Type: v1.UserType_USER_TYPE_USER})
		requireStatusCode(t, err, codes.Internal)
	})
}

func TestUsersServer_ListUsers_GRPC(t *testing.T) {
	ctx := context.Background()

	t.Run("defaults and mapping", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		users := []*model.User{
			{ID: uuid.New(), Name: "u1", Type: model.UserTypeUser, IsActive: true},
			{ID: uuid.New(), Name: "u2", Type: model.UserTypeBot, IsActive: false},
		}
		userRepo.On("ListUsers", mock.Anything, 50, 0).Return(users, nil).Once()

		resp, err := client.ListUsers(ctx, &v1.ListUsersRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetTotal() != int32(len(users)) {
			t.Fatalf("expected total %d, got %d", len(users), resp.GetTotal())
		}
		if len(resp.GetUsers()) != len(users) {
			t.Fatalf("expected %d users, got %d", len(users), len(resp.GetUsers()))
		}
		if resp.GetUsers()[0].GetName() != "u1" {
			t.Fatalf("expected first user name u1, got %s", resp.GetUsers()[0].GetName())
		}
	})

	t.Run("page size capped to 1000", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		userRepo.On("ListUsers", mock.Anything, 1000, 2000).Return([]*model.User{}, nil).Once()

		_, err := client.ListUsers(ctx, &v1.ListUsersRequest{PageSize: 50000, Page: 2})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		userRepo.On("ListUsers", mock.Anything, 50, 0).Return(nil, errors.New("db")).Once()

		_, err := client.ListUsers(ctx, &v1.ListUsersRequest{})
		requireStatusCode(t, err, codes.Internal)
	})

	t.Run("negative page treated as 0", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		userRepo.On("ListUsers", mock.Anything, 50, 0).Return([]*model.User{}, nil).Once()

		_, err := client.ListUsers(ctx, &v1.ListUsersRequest{PageSize: 0, Page: -10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestUsersServer_GetUser_GRPC(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid uuid", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		_, err := client.GetUser(ctx, &v1.GetUserRequest{Id: "not-a-uuid"})
		requireStatusCode(t, err, codes.InvalidArgument)
	})

	t.Run("not found", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		userRepo.On("GetUser", mock.Anything, id).Return((*model.User)(nil), data.ErrUserNotFound).Once()

		_, err := client.GetUser(ctx, &v1.GetUserRequest{Id: id.String()})
		requireStatusCode(t, err, codes.NotFound)
	})

	t.Run("repo error", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		userRepo.On("GetUser", mock.Anything, id).Return((*model.User)(nil), errors.New("db")).Once()

		_, err := client.GetUser(ctx, &v1.GetUserRequest{Id: id.String()})
		requireStatusCode(t, err, codes.Internal)
	})

	t.Run("success", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		userRepo.On("GetUser", mock.Anything, id).Return(&model.User{ID: id, Name: "bob", Type: model.UserTypeUser, IsActive: true}, nil).Once()

		resp, err := client.GetUser(ctx, &v1.GetUserRequest{Id: id.String()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetId() != id.String() {
			t.Fatalf("expected id %s, got %s", id, resp.GetId())
		}
		if resp.GetName() != "bob" {
			t.Fatalf("expected name bob, got %s", resp.GetName())
		}
	})
}

func TestUsersServer_UpdateUser_GRPC(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid uuid", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		_, err := client.UpdateUser(ctx, &v1.UpdateUserRequest{Id: "not-a-uuid", Name: "n", IsActive: true})
		requireStatusCode(t, err, codes.InvalidArgument)
	})

	t.Run("updates fields", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		user := &model.User{ID: id, Name: "old", Type: model.UserTypeUser, IsActive: true}
		userRepo.On("GetUser", mock.Anything, id).Return(user, nil).Once()
		userRepo.On("UpdateUser", mock.Anything, mock.MatchedBy(func(u *model.User) bool {
			return u != nil && u.ID == id && u.Name == "new" && !u.IsActive
		})).Return(nil).Once()

		resp, err := client.UpdateUser(ctx, &v1.UpdateUserRequest{Id: id.String(), Name: "new", IsActive: false})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetName() != "new" {
			t.Fatalf("expected name new, got %s", resp.GetName())
		}
		if resp.GetIsActive() != false {
			t.Fatalf("expected is_active false")
		}
	})

	t.Run("keeps name when empty", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		user := &model.User{ID: id, Name: "keep", Type: model.UserTypeUser, IsActive: true}
		userRepo.On("GetUser", mock.Anything, id).Return(user, nil).Once()
		userRepo.On("UpdateUser", mock.Anything, mock.MatchedBy(func(u *model.User) bool {
			return u != nil && u.ID == id && u.Name == "keep" && !u.IsActive
		})).Return(nil).Once()

		resp, err := client.UpdateUser(ctx, &v1.UpdateUserRequest{Id: id.String(), Name: "", IsActive: false})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetName() != "keep" {
			t.Fatalf("expected name keep, got %s", resp.GetName())
		}
	})

	t.Run("not found", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		userRepo.On("GetUser", mock.Anything, id).Return((*model.User)(nil), data.ErrUserNotFound).Once()

		_, err := client.UpdateUser(ctx, &v1.UpdateUserRequest{Id: id.String(), Name: "x", IsActive: true})
		requireStatusCode(t, err, codes.NotFound)
	})

	t.Run("get error", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		userRepo.On("GetUser", mock.Anything, id).Return((*model.User)(nil), errors.New("db")).Once()

		_, err := client.UpdateUser(ctx, &v1.UpdateUserRequest{Id: id.String(), Name: "x", IsActive: true})
		requireStatusCode(t, err, codes.Internal)
	})

	t.Run("update error", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		user := &model.User{ID: id, Name: "old", Type: model.UserTypeUser, IsActive: true}
		userRepo.On("GetUser", mock.Anything, id).Return(user, nil).Once()
		userRepo.On("UpdateUser", mock.Anything, mock.Anything).Return(errors.New("db")).Once()

		_, err := client.UpdateUser(ctx, &v1.UpdateUserRequest{Id: id.String(), Name: "new", IsActive: true})
		requireStatusCode(t, err, codes.Internal)
	})
}

func TestUsersServer_DeleteUser_GRPC(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid uuid", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		_, err := client.DeleteUser(ctx, &v1.DeleteUserRequest{Id: "not-a-uuid"})
		requireStatusCode(t, err, codes.InvalidArgument)
	})

	t.Run("success", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		aclRepo.On("DeleteUserPermissions", mock.Anything, id).Return(nil).Once()
		userRepo.On("DeleteUser", mock.Anything, id).Return(nil).Once()

		resp, err := client.DeleteUser(ctx, &v1.DeleteUserRequest{Id: id.String()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetId() != id.String() {
			t.Fatalf("expected id %s, got %s", id, resp.GetId())
		}
	})

	t.Run("permission delete error", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		aclRepo.On("DeleteUserPermissions", mock.Anything, id).Return(errors.New("db")).Once()

		_, err := client.DeleteUser(ctx, &v1.DeleteUserRequest{Id: id.String()})
		requireStatusCode(t, err, codes.Internal)
		userRepo.AssertNotCalled(t, "DeleteUser", mock.Anything, mock.Anything)
	})

	t.Run("not found", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		aclRepo.On("DeleteUserPermissions", mock.Anything, id).Return(nil).Once()
		userRepo.On("DeleteUser", mock.Anything, id).Return(data.ErrUserNotFound).Once()

		_, err := client.DeleteUser(ctx, &v1.DeleteUserRequest{Id: id.String()})
		requireStatusCode(t, err, codes.NotFound)
	})

	t.Run("delete error", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		aclRepo.On("DeleteUserPermissions", mock.Anything, id).Return(nil).Once()
		userRepo.On("DeleteUser", mock.Anything, id).Return(errors.New("db")).Once()

		_, err := client.DeleteUser(ctx, &v1.DeleteUserRequest{Id: id.String()})
		requireStatusCode(t, err, codes.Internal)
	})
}

func TestUsersServer_RegenerateToken_GRPC(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid uuid", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		_, err := client.RegenerateToken(ctx, &v1.RegenerateTokenRequest{Id: "not-a-uuid"})
		requireStatusCode(t, err, codes.InvalidArgument)
	})

	t.Run("success", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		user := &model.User{ID: id, Name: "bob", Type: model.UserTypeUser, Token: "pbuf_user_old", IsActive: true}
		userRepo.On("GetUser", mock.Anything, id).Return(user, nil).Once()
		userRepo.On("UpdateUser", mock.Anything, mock.MatchedBy(func(u *model.User) bool {
			return u != nil && u.ID == id && strings.HasPrefix(u.Token, "pbuf_user_") && u.Token != "pbuf_user_old"
		})).Return(nil).Once()

		resp, err := client.RegenerateToken(ctx, &v1.RegenerateTokenRequest{Id: id.String()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetToken() == "" {
			t.Fatalf("expected token to be set")
		}
		if !strings.HasPrefix(resp.GetToken(), "pbuf_user_") {
			t.Fatalf("expected token prefix pbuf_user_, got %s", resp.GetToken())
		}
	})

	t.Run("not found", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		userRepo.On("GetUser", mock.Anything, id).Return((*model.User)(nil), data.ErrUserNotFound).Once()

		_, err := client.RegenerateToken(ctx, &v1.RegenerateTokenRequest{Id: id.String()})
		requireStatusCode(t, err, codes.NotFound)
	})

	t.Run("update error", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		user := &model.User{ID: id, Name: "bob", Type: model.UserTypeUser, Token: "pbuf_user_old", IsActive: true}
		userRepo.On("GetUser", mock.Anything, id).Return(user, nil).Once()
		userRepo.On("UpdateUser", mock.Anything, mock.Anything).Return(errors.New("db")).Once()

		_, err := client.RegenerateToken(ctx, &v1.RegenerateTokenRequest{Id: id.String()})
		requireStatusCode(t, err, codes.Internal)
	})
}

func TestUsersServer_GrantPermission_GRPC(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid uuid", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		_, err := client.GrantPermission(ctx, &v1.GrantPermissionRequest{UserId: "not-a-uuid", ModuleName: "m", Permission: v1.Permission_PERMISSION_READ})
		requireStatusCode(t, err, codes.InvalidArgument)
	})

	t.Run("invalid permission", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		_, err := client.GrantPermission(ctx, &v1.GrantPermissionRequest{UserId: id.String(), ModuleName: "m", Permission: v1.Permission_PERMISSION_UNSPECIFIED})
		requireStatusCode(t, err, codes.InvalidArgument)
	})

	t.Run("success", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		entryID := uuid.New()
		createdAt := time.Now().UTC().Truncate(time.Second)

		userRepo.On("GetUser", mock.Anything, id).Return(&model.User{ID: id}, nil).Once()
		aclRepo.On("GrantPermission", mock.Anything, mock.MatchedBy(func(e *model.ACLEntry) bool {
			return e != nil && e.UserID == id && e.ModuleName == "mod" && e.Permission == model.PermissionRead
		})).Run(func(args mock.Arguments) {
			e := args.Get(1).(*model.ACLEntry)
			e.ID = entryID
			e.CreatedAt = createdAt
		}).Return(nil).Once()

		resp, err := client.GrantPermission(ctx, &v1.GrantPermissionRequest{UserId: id.String(), ModuleName: "mod", Permission: v1.Permission_PERMISSION_READ})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetEntry().GetId() != entryID.String() {
			t.Fatalf("expected entry id %s, got %s", entryID, resp.GetEntry().GetId())
		}
		if resp.GetEntry().GetUserId() != id.String() {
			t.Fatalf("expected entry user_id %s, got %s", id, resp.GetEntry().GetUserId())
		}
		if resp.GetEntry().GetModuleName() != "mod" {
			t.Fatalf("expected module_name mod, got %s", resp.GetEntry().GetModuleName())
		}
		if resp.GetEntry().GetPermission() != v1.Permission_PERMISSION_READ {
			t.Fatalf("expected permission READ, got %v", resp.GetEntry().GetPermission())
		}
		if resp.GetEntry().GetCreatedAt() == nil {
			t.Fatalf("expected created_at to be set")
		}
	})

	t.Run("user not found", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		userRepo.On("GetUser", mock.Anything, id).Return((*model.User)(nil), data.ErrUserNotFound).Once()

		_, err := client.GrantPermission(ctx, &v1.GrantPermissionRequest{UserId: id.String(), ModuleName: "mod", Permission: v1.Permission_PERMISSION_READ})
		requireStatusCode(t, err, codes.NotFound)
	})

	t.Run("get user error", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		userRepo.On("GetUser", mock.Anything, id).Return((*model.User)(nil), errors.New("db")).Once()

		_, err := client.GrantPermission(ctx, &v1.GrantPermissionRequest{UserId: id.String(), ModuleName: "mod", Permission: v1.Permission_PERMISSION_READ})
		requireStatusCode(t, err, codes.Internal)
	})

	t.Run("grant error", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		userRepo.On("GetUser", mock.Anything, id).Return(&model.User{ID: id}, nil).Once()
		aclRepo.On("GrantPermission", mock.Anything, mock.Anything).Return(errors.New("db")).Once()

		_, err := client.GrantPermission(ctx, &v1.GrantPermissionRequest{UserId: id.String(), ModuleName: "mod", Permission: v1.Permission_PERMISSION_READ})
		requireStatusCode(t, err, codes.Internal)
	})
}

func TestUsersServer_RevokePermission_GRPC(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid uuid", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		_, err := client.RevokePermission(ctx, &v1.RevokePermissionRequest{UserId: "not-a-uuid", ModuleName: "mod"})
		requireStatusCode(t, err, codes.InvalidArgument)
	})

	t.Run("permission not found", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		aclRepo.On("RevokePermission", mock.Anything, id, "mod").Return(data.ErrPermissionNotFound).Once()

		_, err := client.RevokePermission(ctx, &v1.RevokePermissionRequest{UserId: id.String(), ModuleName: "mod"})
		requireStatusCode(t, err, codes.NotFound)
	})

	t.Run("success", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		aclRepo.On("RevokePermission", mock.Anything, id, "mod").Return(nil).Once()

		resp, err := client.RevokePermission(ctx, &v1.RevokePermissionRequest{UserId: id.String(), ModuleName: "mod"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.GetSuccess() {
			t.Fatalf("expected success=true")
		}
	})

	t.Run("repo error", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		aclRepo.On("RevokePermission", mock.Anything, id, "mod").Return(errors.New("db")).Once()

		_, err := client.RevokePermission(ctx, &v1.RevokePermissionRequest{UserId: id.String(), ModuleName: "mod"})
		requireStatusCode(t, err, codes.Internal)
	})
}

func TestUsersServer_ListUserPermissions_GRPC(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid uuid", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		_, err := client.ListUserPermissions(ctx, &v1.ListUserPermissionsRequest{UserId: "not-a-uuid"})
		requireStatusCode(t, err, codes.InvalidArgument)
	})

	t.Run("repo error", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		aclRepo.On("ListUserPermissions", mock.Anything, id).Return(nil, errors.New("db")).Once()

		_, err := client.ListUserPermissions(ctx, &v1.ListUserPermissionsRequest{UserId: id.String()})
		requireStatusCode(t, err, codes.Internal)
	})

	t.Run("success", func(t *testing.T) {
		userRepo := mocks.NewUserRepository(t)
		aclRepo := mocks.NewACLRepository(t)
		client, closer := setupUsersClient(t, userRepo, aclRepo)
		defer closer()

		id := uuid.New()
		entries := []*model.ACLEntry{
			{ID: uuid.New(), UserID: id, ModuleName: "m1", Permission: model.PermissionRead, CreatedAt: time.Now()},
			{ID: uuid.New(), UserID: id, ModuleName: "m2", Permission: model.PermissionAdmin, CreatedAt: time.Now()},
		}
		aclRepo.On("ListUserPermissions", mock.Anything, id).Return(entries, nil).Once()

		resp, err := client.ListUserPermissions(ctx, &v1.ListUserPermissionsRequest{UserId: id.String()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.GetPermissions()) != len(entries) {
			t.Fatalf("expected %d entries, got %d", len(entries), len(resp.GetPermissions()))
		}
		if resp.GetPermissions()[0].GetUserId() != id.String() {
			t.Fatalf("expected user_id %s, got %s", id, resp.GetPermissions()[0].GetUserId())
		}
	})
}
