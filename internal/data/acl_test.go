package data

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestUserForACL(t *testing.T, name string) *model.User {
	user := &model.User{
		Name:     name,
		Token:    "pbuf_user_acl-test-" + name,
		Type:     model.UserTypeUser,
		IsActive: true,
	}
	err := suite.userRepository.CreateUser(context.Background(), user)
	require.NoError(t, err)
	return user
}

func Test_aclRepository_GrantPermission(t *testing.T) {
	testUser := createTestUserForACL(t, "acl-grant-user")

	tests := []struct {
		name       string
		entry      *model.ACLEntry
		wantErr    bool
	}{
		{
			name: "Grant read permission on specific module",
			entry: &model.ACLEntry{
				UserID:     testUser.ID,
				ModuleName: "test-module-1",
				Permission: model.PermissionRead,
			},
			wantErr: false,
		},
		{
			name: "Grant write permission on specific module",
			entry: &model.ACLEntry{
				UserID:     testUser.ID,
				ModuleName: "test-module-2",
				Permission: model.PermissionWrite,
			},
			wantErr: false,
		},
		{
			name: "Grant admin permission on wildcard",
			entry: &model.ACLEntry{
				UserID:     testUser.ID,
				ModuleName: "*",
				Permission: model.PermissionAdmin,
			},
			wantErr: false,
		},
		{
			name: "Update existing permission (upsert)",
			entry: &model.ACLEntry{
				UserID:     testUser.ID,
				ModuleName: "test-module-1",
				Permission: model.PermissionWrite,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.aclRepository

			err := r.GrantPermission(context.Background(), tt.entry)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEqual(t, uuid.Nil, tt.entry.ID)
			assert.False(t, tt.entry.CreatedAt.IsZero())
		})
	}
}

func Test_aclRepository_ListUserPermissions(t *testing.T) {
	testUser := createTestUserForACL(t, "acl-list-user")

	// Grant some permissions
	permissions := []struct {
		moduleName string
		permission model.Permission
	}{
		{"list-module-1", model.PermissionRead},
		{"list-module-2", model.PermissionWrite},
		{"list-module-3", model.PermissionAdmin},
	}

	for _, p := range permissions {
		entry := &model.ACLEntry{
			UserID:     testUser.ID,
			ModuleName: p.moduleName,
			Permission: p.permission,
		}
		err := suite.aclRepository.GrantPermission(context.Background(), entry)
		require.NoError(t, err)
	}

	t.Run("List all permissions for user", func(t *testing.T) {
		r := suite.aclRepository

		entries, err := r.ListUserPermissions(context.Background(), testUser.ID)
		require.NoError(t, err)
		assert.Len(t, entries, 3)

		// Verify all permissions are present
		moduleNames := make(map[string]model.Permission)
		for _, e := range entries {
			moduleNames[e.ModuleName] = e.Permission
		}
		assert.Equal(t, model.PermissionRead, moduleNames["list-module-1"])
		assert.Equal(t, model.PermissionWrite, moduleNames["list-module-2"])
		assert.Equal(t, model.PermissionAdmin, moduleNames["list-module-3"])
	})

	t.Run("List permissions for user with no permissions", func(t *testing.T) {
		r := suite.aclRepository

		entries, err := r.ListUserPermissions(context.Background(), uuid.New())
		require.NoError(t, err)
		assert.Empty(t, entries)
	})
}

func Test_aclRepository_CheckPermission(t *testing.T) {
	testUser := createTestUserForACL(t, "acl-check-user")

	// Grant specific permissions
	entries := []*model.ACLEntry{
		{
			UserID:     testUser.ID,
			ModuleName: "check-module-read",
			Permission: model.PermissionRead,
		},
		{
			UserID:     testUser.ID,
			ModuleName: "check-module-write",
			Permission: model.PermissionWrite,
		},
		{
			UserID:     testUser.ID,
			ModuleName: "check-module-admin",
			Permission: model.PermissionAdmin,
		},
	}

	for _, entry := range entries {
		err := suite.aclRepository.GrantPermission(context.Background(), entry)
		require.NoError(t, err)
	}

	tests := []struct {
		name       string
		moduleName string
		permission model.Permission
		wantHas    bool
	}{
		{
			name:       "Check read permission on read module",
			moduleName: "check-module-read",
			permission: model.PermissionRead,
			wantHas:    true,
		},
		{
			name:       "Check write permission on read module (should fail)",
			moduleName: "check-module-read",
			permission: model.PermissionWrite,
			wantHas:    false,
		},
		{
			name:       "Check read permission on write module (should pass - higher grants lower)",
			moduleName: "check-module-write",
			permission: model.PermissionRead,
			wantHas:    true,
		},
		{
			name:       "Check write permission on write module",
			moduleName: "check-module-write",
			permission: model.PermissionWrite,
			wantHas:    true,
		},
		{
			name:       "Check admin permission on write module (should fail)",
			moduleName: "check-module-write",
			permission: model.PermissionAdmin,
			wantHas:    false,
		},
		{
			name:       "Check admin permission on admin module",
			moduleName: "check-module-admin",
			permission: model.PermissionAdmin,
			wantHas:    true,
		},
		{
			name:       "Check read permission on admin module (should pass)",
			moduleName: "check-module-admin",
			permission: model.PermissionRead,
			wantHas:    true,
		},
		{
			name:       "Check permission on non-existing module",
			moduleName: "non-existing-module",
			permission: model.PermissionRead,
			wantHas:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.aclRepository

			hasPermission, err := r.CheckPermission(context.Background(), testUser.ID, tt.moduleName, tt.permission)
			require.NoError(t, err)
			assert.Equal(t, tt.wantHas, hasPermission)
		})
	}
}

func Test_aclRepository_CheckPermission_Wildcard(t *testing.T) {
	testUser := createTestUserForACL(t, "acl-wildcard-user")

	// Grant wildcard write permission
	entry := &model.ACLEntry{
		UserID:     testUser.ID,
		ModuleName: "*",
		Permission: model.PermissionWrite,
	}
	err := suite.aclRepository.GrantPermission(context.Background(), entry)
	require.NoError(t, err)

	tests := []struct {
		name       string
		moduleName string
		permission model.Permission
		wantHas    bool
	}{
		{
			name:       "Check read permission on any module (wildcard grants)",
			moduleName: "any-module-1",
			permission: model.PermissionRead,
			wantHas:    true,
		},
		{
			name:       "Check write permission on any module (wildcard grants)",
			moduleName: "any-module-2",
			permission: model.PermissionWrite,
			wantHas:    true,
		},
		{
			name:       "Check admin permission on any module (wildcard doesn't grant admin)",
			moduleName: "any-module-3",
			permission: model.PermissionAdmin,
			wantHas:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.aclRepository

			hasPermission, err := r.CheckPermission(context.Background(), testUser.ID, tt.moduleName, tt.permission)
			require.NoError(t, err)
			assert.Equal(t, tt.wantHas, hasPermission)
		})
	}
}

func Test_aclRepository_CheckPermission_SpecificOverridesWildcard(t *testing.T) {
	testUser := createTestUserForACL(t, "acl-specific-override-user")

	// Grant wildcard read permission
	wildcardEntry := &model.ACLEntry{
		UserID:     testUser.ID,
		ModuleName: "*",
		Permission: model.PermissionRead,
	}
	err := suite.aclRepository.GrantPermission(context.Background(), wildcardEntry)
	require.NoError(t, err)

	// Grant specific admin permission on one module
	specificEntry := &model.ACLEntry{
		UserID:     testUser.ID,
		ModuleName: "specific-admin-module",
		Permission: model.PermissionAdmin,
	}
	err = suite.aclRepository.GrantPermission(context.Background(), specificEntry)
	require.NoError(t, err)

	t.Run("Specific permission overrides wildcard", func(t *testing.T) {
		r := suite.aclRepository

		// Check admin on specific module should pass
		hasPermission, err := r.CheckPermission(context.Background(), testUser.ID, "specific-admin-module", model.PermissionAdmin)
		require.NoError(t, err)
		assert.True(t, hasPermission)

		// Check admin on other module should fail (only wildcard read)
		hasPermission, err = r.CheckPermission(context.Background(), testUser.ID, "other-module", model.PermissionAdmin)
		require.NoError(t, err)
		assert.False(t, hasPermission)

		// Check read on other module should pass (wildcard read)
		hasPermission, err = r.CheckPermission(context.Background(), testUser.ID, "other-module", model.PermissionRead)
		require.NoError(t, err)
		assert.True(t, hasPermission)
	})
}

func Test_aclRepository_RevokePermission(t *testing.T) {
	testUser := createTestUserForACL(t, "acl-revoke-user")

	// Grant a permission first
	entry := &model.ACLEntry{
		UserID:     testUser.ID,
		ModuleName: "revoke-test-module",
		Permission: model.PermissionWrite,
	}
	err := suite.aclRepository.GrantPermission(context.Background(), entry)
	require.NoError(t, err)

	tests := []struct {
		name       string
		userID     uuid.UUID
		moduleName string
		wantErr    bool
		errType    error
	}{
		{
			name:       "Revoke existing permission",
			userID:     testUser.ID,
			moduleName: "revoke-test-module",
			wantErr:    false,
		},
		{
			name:       "Revoke non-existing permission",
			userID:     testUser.ID,
			moduleName: "non-existing-module",
			wantErr:    true,
			errType:    ErrPermissionNotFound,
		},
		{
			name:       "Revoke already revoked permission",
			userID:     testUser.ID,
			moduleName: "revoke-test-module",
			wantErr:    true,
			errType:    ErrPermissionNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.aclRepository

			err := r.RevokePermission(context.Background(), tt.userID, tt.moduleName)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
				return
			}

			require.NoError(t, err)

			// Verify permission is revoked
			hasPermission, err := r.CheckPermission(context.Background(), tt.userID, tt.moduleName, model.PermissionRead)
			require.NoError(t, err)
			assert.False(t, hasPermission)
		})
	}
}

func Test_aclRepository_DeleteUserPermissions(t *testing.T) {
	testUser := createTestUserForACL(t, "acl-delete-all-user")

	// Grant multiple permissions
	modules := []string{"delete-module-1", "delete-module-2", "delete-module-3"}
	for _, m := range modules {
		entry := &model.ACLEntry{
			UserID:     testUser.ID,
			ModuleName: m,
			Permission: model.PermissionRead,
		}
		err := suite.aclRepository.GrantPermission(context.Background(), entry)
		require.NoError(t, err)
	}

	// Verify permissions exist
	entries, err := suite.aclRepository.ListUserPermissions(context.Background(), testUser.ID)
	require.NoError(t, err)
	assert.Len(t, entries, 3)

	t.Run("Delete all user permissions", func(t *testing.T) {
		r := suite.aclRepository

		err := r.DeleteUserPermissions(context.Background(), testUser.ID)
		require.NoError(t, err)

		// Verify all permissions are deleted
		entries, err := r.ListUserPermissions(context.Background(), testUser.ID)
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("Delete permissions for user with no permissions", func(t *testing.T) {
		r := suite.aclRepository

		// Should not error even if user has no permissions
		err := r.DeleteUserPermissions(context.Background(), uuid.New())
		require.NoError(t, err)
	})
}

func Test_aclRepository_CascadeDeleteOnUserDelete(t *testing.T) {
	testUser := createTestUserForACL(t, "acl-cascade-user")

	// Grant permission
	entry := &model.ACLEntry{
		UserID:     testUser.ID,
		ModuleName: "cascade-module",
		Permission: model.PermissionWrite,
	}
	err := suite.aclRepository.GrantPermission(context.Background(), entry)
	require.NoError(t, err)

	// Verify permission exists
	entries, err := suite.aclRepository.ListUserPermissions(context.Background(), testUser.ID)
	require.NoError(t, err)
	assert.Len(t, entries, 1)

	// Delete user
	err = suite.userRepository.DeleteUser(context.Background(), testUser.ID)
	require.NoError(t, err)

	// Verify permissions are cascade deleted
	entries, err = suite.aclRepository.ListUserPermissions(context.Background(), testUser.ID)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
