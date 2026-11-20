package data

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pbufio/pbuf-registry/internal/model"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
)

const (
	tokenCacheTTL        = 5 * time.Second
	maxTokenCacheEntries = 1024
)

type cachedTokenUser struct {
	user      model.User
	expiresAt time.Time
}

type UserRepository interface {
	CreateUser(ctx context.Context, user *model.User) error
	GetUser(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetUserByName(ctx context.Context, name string) (*model.User, error)
	GetUserByToken(ctx context.Context, token string) (*model.User, error)
	ListUsers(ctx context.Context, limit int, offset int) ([]*model.User, int, error)
	UpdateUser(ctx context.Context, user *model.User) error
	DeleteUser(ctx context.Context, id uuid.UUID) error
}

type userRepo struct {
	pool   *pgxpool.Pool
	logger *log.Helper

	// shortcut: per-process 5s SHA-256 token cache to amortize bcrypt CPU under bursts, move to shared cache if instant cross-replica revocation is needed
	cacheMu    sync.RWMutex
	cacheGen   uint64
	tokenCache map[[32]byte]cachedTokenUser
	nameCache  map[string]cachedTokenUser
}

func NewUserRepository(pool *pgxpool.Pool, logger log.Logger) UserRepository {
	return &userRepo{
		pool:       pool,
		logger:     log.NewHelper(log.With(logger, "module", "data/UserRepository")),
		tokenCache: make(map[[32]byte]cachedTokenUser),
		nameCache:  make(map[string]cachedTokenUser),
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func putCachedUser[K comparable](m map[K]cachedTokenUser, key K, u model.User, now time.Time) {
	if len(m) >= maxTokenCacheEntries {
		for k, entry := range m {
			if !now.Before(entry.expiresAt) {
				delete(m, k)
			}
		}
		if len(m) >= maxTokenCacheEntries {
			clear(m)
		}
	}
	m[key] = cachedTokenUser{
		user:      u,
		expiresAt: now.Add(tokenCacheTTL),
	}
}

func (r *userRepo) evictUserCache(userID uuid.UUID) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	r.cacheGen++
	for k, entry := range r.tokenCache {
		if entry.user.ID == userID {
			delete(r.tokenCache, k)
		}
	}
	for k, entry := range r.nameCache {
		if entry.user.ID == userID {
			delete(r.nameCache, k)
		}
	}
}

func (r *userRepo) CreateUser(ctx context.Context, user *model.User) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (name, token, token_lookup, type, is_active, created_at, updated_at)
		VALUES ($1, crypt($2, gen_salt('bf')), encode(digest($2, 'sha256'), 'hex'), $3, $4, NOW(), NOW())
		RETURNING id, created_at, updated_at`,
		user.Name, user.Token, user.Type, user.IsActive,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrUserAlreadyExists
		}
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
	now := time.Now()
	r.cacheMu.RLock()
	gen := r.cacheGen
	if cached, ok := r.nameCache[name]; ok && now.Before(cached.expiresAt) {
		u := cached.user
		r.cacheMu.RUnlock()
		return &u, nil
	}
	r.cacheMu.RUnlock()

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

	r.cacheMu.Lock()
	if r.cacheGen == gen {
		putCachedUser(r.nameCache, name, user, now)
	}
	r.cacheMu.Unlock()

	return &user, nil
}

func (r *userRepo) GetUserByToken(ctx context.Context, token string) (*model.User, error) {
	tokenSum := sha256.Sum256([]byte(token))
	now := time.Now()

	r.cacheMu.RLock()
	gen := r.cacheGen
	if cached, ok := r.tokenCache[tokenSum]; ok && now.Before(cached.expiresAt) {
		u := cached.user
		r.cacheMu.RUnlock()
		return &u, nil
	}
	r.cacheMu.RUnlock()

	lookupHex := hex.EncodeToString(tokenSum[:])
	var (
		user        model.User
		tokenLookup *string
	)
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, token, token_lookup, type, is_active, created_at, updated_at
		FROM users
		WHERE (token_lookup = $2 OR token_lookup IS NULL)
		  AND (token = crypt($1, token)) = true
		  AND is_active = true`,
		token, lookupHex,
	).Scan(&user.ID, &user.Name, &user.Token, &tokenLookup, &user.Type, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("could not select user from database: %w", err)
	}

	if tokenLookup == nil {
		if _, backfillErr := r.pool.Exec(ctx,
			`UPDATE users SET token_lookup = $1 WHERE id = $2 AND token_lookup IS NULL`,
			lookupHex, user.ID,
		); backfillErr != nil {
			r.logger.Warnf("failed to backfill token_lookup for user %s: %v", user.ID, backfillErr)
		}
	}

	r.cacheMu.Lock()
	if r.cacheGen == gen {
		putCachedUser(r.tokenCache, tokenSum, user, now)
	}
	r.cacheMu.Unlock()

	return &user, nil
}

func (r *userRepo) ListUsers(ctx context.Context, limit int, offset int) ([]*model.User, int, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, token, type, is_active, created_at, updated_at, COUNT(*) OVER()
		FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("could not select users from database: %w", err)
	}
	defer rows.Close()

	var (
		users []*model.User
		total int
	)
	for rows.Next() {
		var user model.User
		err := rows.Scan(&user.ID, &user.Name, &user.Token, &user.Type, &user.IsActive, &user.CreatedAt, &user.UpdatedAt, &total)
		if err != nil {
			return nil, 0, fmt.Errorf("could not scan user: %w", err)
		}
		users = append(users, &user)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("could not iterate users: %w", err)
	}
	rows.Close()

	if len(users) == 0 && offset > 0 {
		if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("could not count users in database: %w", err)
		}
	}

	return users, total, nil
}

func (r *userRepo) UpdateUser(ctx context.Context, user *model.User) error {
	var (
		query string
		args  []any
	)
	if user.Token == "" || strings.HasPrefix(user.Token, "$2") {
		query = `UPDATE users SET name = $1, type = $2, is_active = $3, updated_at = NOW() WHERE id = $4`
		args = []any{user.Name, user.Type, user.IsActive, user.ID}
	} else {
		query = `UPDATE users SET name = $1, token = crypt($2, gen_salt('bf')), token_lookup = encode(digest($2, 'sha256'), 'hex'), type = $3, is_active = $4, updated_at = NOW() WHERE id = $5`
		args = []any{user.Name, user.Token, user.Type, user.IsActive, user.ID}
	}
	result, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrUserAlreadyExists
		}
		return fmt.Errorf("could not update user in database: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	r.evictUserCache(user.ID)
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
	r.evictUserCache(id)
	return nil
}
