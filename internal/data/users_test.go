package data

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_userRepository_CreateUser(t *testing.T) {
	tests := []struct {
		name    string
		user    *model.User
		wantErr bool
	}{
		{
			name: "Create user",
			user: &model.User{
				Name:     "test-user-1",
				Token:    "pbuf_user_test-token-1",
				Type:     model.UserTypeUser,
				IsActive: true,
			},
			wantErr: false,
		},
		{
			name: "Create bot",
			user: &model.User{
				Name:     "test-bot-1",
				Token:    "pbuf_bot_test-token-2",
				Type:     model.UserTypeBot,
				IsActive: true,
			},
			wantErr: false,
		},
		{
			name: "Create user with duplicate name",
			user: &model.User{
				Name:     "test-user-1",
				Token:    "pbuf_user_test-token-3",
				Type:     model.UserTypeUser,
				IsActive: true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.userRepository

			err := r.CreateUser(context.Background(), tt.user)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEqual(t, uuid.Nil, tt.user.ID)
			assert.False(t, tt.user.CreatedAt.IsZero())
			assert.False(t, tt.user.UpdatedAt.IsZero())
		})
	}
}

func Test_userRepository_GetUser(t *testing.T) {
	// First create a user to get
	testUser := &model.User{
		Name:     "test-get-user",
		Token:    "pbuf_user_get-token",
		Type:     model.UserTypeUser,
		IsActive: true,
	}
	err := suite.userRepository.CreateUser(context.Background(), testUser)
	require.NoError(t, err)

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr bool
		errType error
	}{
		{
			name:    "Get existing user",
			id:      testUser.ID,
			wantErr: false,
		},
		{
			name:    "Get non-existing user",
			id:      uuid.New(),
			wantErr: true,
			errType: ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.userRepository

			user, err := r.GetUser(context.Background(), tt.id)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, testUser.ID, user.ID)
			assert.Equal(t, testUser.Name, user.Name)
			assert.Equal(t, testUser.Type, user.Type)
			assert.Equal(t, testUser.IsActive, user.IsActive)
		})
	}
}

func Test_userRepository_GetUserByName(t *testing.T) {
	// First create a user to get
	testUser := &model.User{
		Name:     "test-get-by-name-user",
		Token:    "pbuf_user_get-by-name-token",
		Type:     model.UserTypeUser,
		IsActive: true,
	}
	err := suite.userRepository.CreateUser(context.Background(), testUser)
	require.NoError(t, err)

	tests := []struct {
		name     string
		userName string
		wantErr  bool
		errType  error
	}{
		{
			name:     "Get existing user by name",
			userName: testUser.Name,
			wantErr:  false,
		},
		{
			name:     "Get non-existing user by name",
			userName: "non-existing-user",
			wantErr:  true,
			errType:  ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.userRepository

			user, err := r.GetUserByName(context.Background(), tt.userName)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, testUser.ID, user.ID)
			assert.Equal(t, testUser.Name, user.Name)
		})
	}
}

func Test_userRepository_GetUserByToken(t *testing.T) {
	// Create a user with a known token for testing pgcrypto verification
	plainToken := "pbuf_user_pgcrypto-test-token-123"
	testUser := &model.User{
		Name:     "test-token-user",
		Token:    plainToken,
		Type:     model.UserTypeUser,
		IsActive: true,
	}
	err := suite.userRepository.CreateUser(context.Background(), testUser)
	require.NoError(t, err)

	// Create an inactive user
	inactiveToken := "pbuf_user_inactive-token-456"
	inactiveUser := &model.User{
		Name:     "test-inactive-user",
		Token:    inactiveToken,
		Type:     model.UserTypeUser,
		IsActive: false,
	}
	err = suite.userRepository.CreateUser(context.Background(), inactiveUser)
	require.NoError(t, err)

	tests := []struct {
		name    string
		token   string
		wantErr bool
		errType error
	}{
		{
			name:    "Get user by valid token (pgcrypto verification)",
			token:   plainToken,
			wantErr: false,
		},
		{
			name:    "Get user by invalid token",
			token:   "invalid-token",
			wantErr: true,
			errType: ErrUserNotFound,
		},
		{
			name:    "Get inactive user by token",
			token:   inactiveToken,
			wantErr: true,
			errType: ErrUserNotFound,
		},
		{
			name:    "Get user by wrong token",
			token:   "pbuf_user_wrong-token",
			wantErr: true,
			errType: ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.userRepository

			user, err := r.GetUserByToken(context.Background(), tt.token)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, testUser.ID, user.ID)
			assert.Equal(t, testUser.Name, user.Name)
			assert.True(t, user.IsActive)
		})
	}
}

func Test_userRepository_ListUsers(t *testing.T) {
	r := suite.userRepository

	// Get initial count
	users, err := r.ListUsers(context.Background(), 100, 0)
	require.NoError(t, err)
	initialCount := len(users)

	// Create some users for listing
	for i := 0; i < 3; i++ {
		user := &model.User{
			Name:     "list-test-user-" + string(rune('a'+i)),
			Token:    "pbuf_user_list-token-" + string(rune('a'+i)),
			Type:     model.UserTypeUser,
			IsActive: true,
		}
		err := r.CreateUser(context.Background(), user)
		require.NoError(t, err)
	}

	tests := []struct {
		name      string
		limit     int
		offset    int
		wantCount int
	}{
		{
			name:      "List all users",
			limit:     100,
			offset:    0,
			wantCount: initialCount + 3,
		},
		{
			name:      "List users with limit",
			limit:     2,
			offset:    0,
			wantCount: 2,
		},
		{
			name:      "List users with offset",
			limit:     100,
			offset:    initialCount + 2,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users, err := r.ListUsers(context.Background(), tt.limit, tt.offset)
			require.NoError(t, err)
			assert.Len(t, users, tt.wantCount)
		})
	}
}

func Test_userRepository_UpdateUser(t *testing.T) {
	// Create a user to update
	testUser := &model.User{
		Name:     "test-update-user",
		Token:    "pbuf_user_update-token",
		Type:     model.UserTypeUser,
		IsActive: true,
	}
	err := suite.userRepository.CreateUser(context.Background(), testUser)
	require.NoError(t, err)

	originalToken := testUser.Token

	tests := []struct {
		name       string
		updateFunc func(u *model.User)
		wantErr    bool
	}{
		{
			name: "Update user name",
			updateFunc: func(u *model.User) {
				u.Name = "updated-user-name"
			},
			wantErr: false,
		},
		{
			name: "Update user active status",
			updateFunc: func(u *model.User) {
				u.IsActive = false
			},
			wantErr: false,
		},
		{
			name: "Update user token",
			updateFunc: func(u *model.User) {
				u.Token = "pbuf_user_new-token"
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.userRepository

			// Apply update
			tt.updateFunc(testUser)

			err := r.UpdateUser(context.Background(), testUser)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify update
			updated, err := r.GetUser(context.Background(), testUser.ID)
			require.NoError(t, err)
			assert.Equal(t, testUser.Name, updated.Name)
			assert.Equal(t, testUser.IsActive, updated.IsActive)
		})
	}

	// Verify that new token works and old token doesn't
	t.Run("Verify token update with pgcrypto", func(t *testing.T) {
		r := suite.userRepository

		// Old token should not work
		_, err := r.GetUserByToken(context.Background(), originalToken)
		assert.ErrorIs(t, err, ErrUserNotFound)

		// New token should work (need to reactivate user first)
		testUser.IsActive = true
		err = r.UpdateUser(context.Background(), testUser)
		require.NoError(t, err)

		user, err := r.GetUserByToken(context.Background(), "pbuf_user_new-token")
		require.NoError(t, err)
		assert.Equal(t, testUser.ID, user.ID)
	})
}

func Test_userRepository_SetUserActive(t *testing.T) {
	// Create a user
	testUser := &model.User{
		Name:     "test-set-active-user",
		Token:    "pbuf_user_set-active-token",
		Type:     model.UserTypeUser,
		IsActive: true,
	}
	err := suite.userRepository.CreateUser(context.Background(), testUser)
	require.NoError(t, err)

	tests := []struct {
		name     string
		isActive bool
	}{
		{
			name:     "Deactivate user",
			isActive: false,
		},
		{
			name:     "Activate user",
			isActive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.userRepository

			err := r.SetUserActive(context.Background(), testUser.ID, tt.isActive)
			require.NoError(t, err)

			// Verify
			user, err := r.GetUser(context.Background(), testUser.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.isActive, user.IsActive)
		})
	}

	t.Run("Set active for non-existing user", func(t *testing.T) {
		r := suite.userRepository

		err := r.SetUserActive(context.Background(), uuid.New(), true)
		assert.ErrorIs(t, err, ErrUserNotFound)
	})
}

func Test_userRepository_DeleteUser(t *testing.T) {
	// Create a user to delete
	testUser := &model.User{
		Name:     "test-delete-user",
		Token:    "pbuf_user_delete-token",
		Type:     model.UserTypeUser,
		IsActive: true,
	}
	err := suite.userRepository.CreateUser(context.Background(), testUser)
	require.NoError(t, err)

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr bool
		errType error
	}{
		{
			name:    "Delete existing user",
			id:      testUser.ID,
			wantErr: false,
		},
		{
			name:    "Delete non-existing user",
			id:      uuid.New(),
			wantErr: true,
			errType: ErrUserNotFound,
		},
		{
			name:    "Delete already deleted user",
			id:      testUser.ID,
			wantErr: true,
			errType: ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.userRepository

			err := r.DeleteUser(context.Background(), tt.id)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
				return
			}

			require.NoError(t, err)

			// Verify deletion
			_, err = r.GetUser(context.Background(), tt.id)
			assert.ErrorIs(t, err, ErrUserNotFound)
		})
	}
}
