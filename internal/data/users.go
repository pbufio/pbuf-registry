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

var ErrUserNotFound = errors.New("user not found")

type UserRepository interface {
	CreateUser(ctx context.Context, user *model.User) error
	GetUser(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetUserByName(ctx context.Context, name string) (*model.User, error)
	GetUserByToken(ctx context.Context, token string) (*model.User, error)
	ListUsers(ctx context.Context, limit int, offset int) ([]*model.User, error)
	UpdateUser(ctx context.Context, user *model.User) error
	DeleteUser(ctx context.Context, id uuid.UUID) error
	SetUserActive(ctx context.Context, id uuid.UUID, isActive bool) error
}

type userRepo struct {
	pool   *pgxpool.Pool
	logger *log.Helper
}

func NewUserRepository(pool *pgxpool.Pool, logger log.Logger) UserRepository {
	return &userRepo{
		pool:   pool,
		logger: log.NewHelper(log.With(logger, "module", "data/UserRepository")),
	}
}

func (r *userRepo) CreateUser(ctx context.Context, user *model.User) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (name, token, type, is_active, created_at, updated_at)
		VALUES ($1, crypt($2, gen_salt('bf')), $3, $4, NOW(), NOW())
		RETURNING id, created_at, updated_at`,
		user.Name, user.Token, user.Type, user.IsActive,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("could not insert user into database: %w", err)
	}
	return nil
}

func (r *userRepo) GetUser(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, token, type, is_active, created_at, updated_at
		FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Name, &user.Token, &user.Type, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("could not select user from database: %w", err)
	}
	return &user, nil
}

func (r *userRepo) GetUserByName(ctx context.Context, name string) (*model.User, error) {
	var user model.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, token, type, is_active, created_at, updated_at
		FROM users WHERE name = $1`,
		name,
	).Scan(&user.ID, &user.Name, &user.Token, &user.Type, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("could not select user from database: %w", err)
	}
	return &user, nil
}

func (r *userRepo) GetUserByToken(ctx context.Context, token string) (*model.User, error) {
	var user model.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, token, type, is_active, created_at, updated_at
		FROM users WHERE (token = crypt($1, token)) = true AND is_active = true`,
		token,
	).Scan(&user.ID, &user.Name, &user.Token, &user.Type, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("could not select user from database: %w", err)
	}
	return &user, nil
}

func (r *userRepo) ListUsers(ctx context.Context, limit int, offset int) ([]*model.User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, token, type, is_active, created_at, updated_at
		FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("could not select users from database: %w", err)
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		var user model.User
		err := rows.Scan(&user.ID, &user.Name, &user.Token, &user.Type, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("could not scan user: %w", err)
		}
		users = append(users, &user)
	}

	return users, nil
}

func (r *userRepo) UpdateUser(ctx context.Context, user *model.User) error {
	result, err := r.pool.Exec(ctx,
		`UPDATE users SET name = $1, token = crypt($2, gen_salt('bf')), type = $3, is_active = $4, updated_at = NOW()
		WHERE id = $5`,
		user.Name, user.Token, user.Type, user.IsActive, user.ID,
	)
	if err != nil {
		return fmt.Errorf("could not update user in database: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *userRepo) DeleteUser(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx,
		`DELETE FROM users WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("could not delete user from database: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *userRepo) SetUserActive(ctx context.Context, id uuid.UUID, isActive bool) error {
	result, err := r.pool.Exec(ctx,
		`UPDATE users SET is_active = $1, updated_at = NOW() WHERE id = $2`,
		isActive, id,
	)
	if err != nil {
		return fmt.Errorf("could not update user active status in database: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}
