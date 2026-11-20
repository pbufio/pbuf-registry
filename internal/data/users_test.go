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
				assert.ErrorIs(t, err, ErrUserAlreadyExists)
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
	users, initialTotal, err := r.ListUsers(context.Background(), 100, 0)
	require.NoError(t, err)
	initialCount := len(users)
	assert.Equal(t, initialCount, initialTotal)

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
		{
			name:      "List users with out-of-range offset still returns total",
			limit:     100,
			offset:    initialCount + 10,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users, total, err := r.ListUsers(context.Background(), tt.limit, tt.offset)
			require.NoError(t, err)
			assert.Len(t, users, tt.wantCount)
			assert.Equal(t, initialCount+3, total)
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

		// Updating a user fetched via GetUser (where Token is a bcrypt hash) must preserve their token
		fetched, err := r.GetUser(context.Background(), testUser.ID)
		require.NoError(t, err)
		fetched.Name = "renamed-after-get"
		require.NoError(t, r.UpdateUser(context.Background(), fetched))

		stillValid, err := r.GetUserByToken(context.Background(), "pbuf_user_new-token")
		require.NoError(t, err)
		assert.Equal(t, "renamed-after-get", stillValid.Name)
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

func Test_userRepository_TokenBackfillCacheAndEviction(t *testing.T) {
	ctx := context.Background()
	r := suite.userRepository

	plainToken := "pbuf_user_legacy-null-lookup-token"
	u := &model.User{
		Name:     "legacy-null-lookup-user",
		Token:    plainToken,
		Type:     model.UserTypeUser,
		IsActive: true,
	}
	require.NoError(t, r.CreateUser(ctx, u))

	// Simulate pre-migration user with NULL token_lookup
	_, err := suite.pool.Exec(ctx, `UPDATE users SET token_lookup = NULL WHERE id = $1`, u.ID)
	require.NoError(t, err)

	// First GetUserByToken should verify via bcrypt, backfill token_lookup, and populate cache
	got, err := r.GetUserByToken(ctx, plainToken)
	require.NoError(t, err)
	assert.Equal(t, u.ID, got.ID)

	var storedLookup *string
	require.NoError(t, suite.pool.QueryRow(ctx, `SELECT token_lookup FROM users WHERE id = $1`, u.ID).Scan(&storedLookup))
	require.NotNil(t, storedLookup)
	assert.Len(t, *storedLookup, 64)

	// Mutating returned pointer must not corrupt cached entry
	got.Name = "mutated-by-caller"
	cachedHit, err := r.GetUserByToken(ctx, plainToken)
	require.NoError(t, err)
	assert.Equal(t, "legacy-null-lookup-user", cachedHit.Name)

	// Duplicate rename must return ErrUserAlreadyExists
	other := &model.User{
		Name:     "collision-target-user",
		Token:    "pbuf_user_collision-tok",
		Type:     model.UserTypeUser,
		IsActive: true,
	}
	require.NoError(t, r.CreateUser(ctx, other))
	other.Name = "legacy-null-lookup-user"
	assert.ErrorIs(t, r.UpdateUser(ctx, other), ErrUserAlreadyExists)

	// Warm GetUserByName cache as well
	byName, err := r.GetUserByName(ctx, "legacy-null-lookup-user")
	require.NoError(t, err)
	assert.True(t, byName.IsActive)

	// Deactivating user via UpdateUser must evict cached token and name immediately
	u.IsActive = false
	require.NoError(t, r.UpdateUser(ctx, u))
	_, err = r.GetUserByToken(ctx, plainToken)
	assert.ErrorIs(t, err, ErrUserNotFound)
	byNameAfterDeactivate, err := r.GetUserByName(ctx, "legacy-null-lookup-user")
	require.NoError(t, err)
	assert.False(t, byNameAfterDeactivate.IsActive)

	// Reactivate, warm both caches, then DeleteUser must evict both immediately
	u.IsActive = true
	require.NoError(t, r.UpdateUser(ctx, u))
	_, err = r.GetUserByToken(ctx, plainToken)
	require.NoError(t, err)
	_, err = r.GetUserByName(ctx, "legacy-null-lookup-user")
	require.NoError(t, err)
	require.NoError(t, r.DeleteUser(ctx, u.ID))
	_, err = r.GetUserByToken(ctx, plainToken)
	assert.ErrorIs(t, err, ErrUserNotFound)
	_, err = r.GetUserByName(ctx, "legacy-null-lookup-user")
	assert.ErrorIs(t, err, ErrUserNotFound)
}
