package data

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pbufio/pbuf-registry/internal/model"
)

var ErrPermissionNotFound = errors.New("permission not found")

type ACLRepository interface {
	GrantPermission(ctx context.Context, entry *model.ACLEntry) error
	RevokePermission(ctx context.Context, userID uuid.UUID, moduleName string) error
	ListUserPermissions(ctx context.Context, userID uuid.UUID) ([]*model.ACLEntry, error)
	CheckPermission(ctx context.Context, userID uuid.UUID, moduleName string, permission model.Permission) (bool, error)
	DeleteUserPermissions(ctx context.Context, userID uuid.UUID) error
}

type aclRepo struct {
	pool   *pgxpool.Pool
	logger *log.Helper
}

func NewACLRepository(pool *pgxpool.Pool, logger log.Logger) ACLRepository {
	return &aclRepo{
		pool:   pool,
		logger: log.NewHelper(log.With(logger, "module", "data/ACLRepository")),
	}
}

func (r *aclRepo) GrantPermission(ctx context.Context, entry *model.ACLEntry) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO acl (user_id, module_name, permission, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, module_name) DO UPDATE SET permission = $3
		RETURNING id, created_at`,
		entry.UserID, entry.ModuleName, entry.Permission,
	).Scan(&entry.ID, &entry.CreatedAt)
	if err != nil {
		return fmt.Errorf("could not grant permission in database: %w", err)
	}
	return nil
}

func (r *aclRepo) RevokePermission(ctx context.Context, userID uuid.UUID, moduleName string) error {
	result, err := r.pool.Exec(ctx,
		`DELETE FROM acl WHERE user_id = $1 AND module_name = $2`,
		userID, moduleName,
	)
	if err != nil {
		return fmt.Errorf("could not revoke permission from database: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrPermissionNotFound
	}
	return nil
}

func (r *aclRepo) ListUserPermissions(ctx context.Context, userID uuid.UUID) ([]*model.ACLEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, module_name, permission, created_at
		FROM acl WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("could not select permissions from database: %w", err)
	}
	defer rows.Close()

	var entries []*model.ACLEntry
	for rows.Next() {
		var entry model.ACLEntry
		err := rows.Scan(&entry.ID, &entry.UserID, &entry.ModuleName, &entry.Permission, &entry.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("could not scan permission: %w", err)
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}

func (r *aclRepo) CheckPermission(ctx context.Context, userID uuid.UUID, moduleName string, permission model.Permission) (bool, error) {
	// Check for specific module permission
	var foundPermission model.Permission
	err := r.pool.QueryRow(ctx,
		`SELECT permission FROM acl WHERE user_id = $1 AND module_name = $2`,
		userID, moduleName,
	).Scan(&foundPermission)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("could not check permission in database: %w", err)
	}
	if err == nil {
		// Found exact module permission, check if it meets required level
		return hasRequiredPermission(foundPermission, permission), nil
	}

	// Check for wildcard permission
	err = r.pool.QueryRow(ctx,
		`SELECT permission FROM acl WHERE user_id = $1 AND module_name = '*'`,
		userID,
	).Scan(&foundPermission)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("could not check wildcard permission in database: %w", err)
	}

	return hasRequiredPermission(foundPermission, permission), nil
}

func (r *aclRepo) DeleteUserPermissions(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM acl WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("could not delete user permissions from database: %w", err)
	}
	return nil
}

// hasRequiredPermission checks if the granted permission meets the required permission level
// admin > write > read
func hasRequiredPermission(granted, required model.Permission) bool {
	permissionLevel := map[model.Permission]int{
		model.PermissionRead:  1,
		model.PermissionWrite: 2,
		model.PermissionAdmin: 3,
	}
	return permissionLevel[granted] >= permissionLevel[required]
}
