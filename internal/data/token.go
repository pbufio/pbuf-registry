package data

import (
	"context"
	"fmt"
	"regexp"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// name of the root token
	rootTokenName = "root"

	// token roles
	roleRoot      TokenRole = "root"
	roleAdmin     TokenRole = "admin"
	roleDeveloper TokenRole = "developer"

	// TokenLength is the size of tokens we are currently generating, without
	// any meta information
	TokenLength = 24

	// MaxRetryTokensGenerationCounter is the maximum number of retries the tokenRepo
	// will make when attempting to get the token
	MaxRetryTokensGenerationCounter = 3
)

type TokenRepository interface {
	RegisterToken(ctx context.Context, token, name, parentToken string) error
	RevokeToken(ctx context.Context, token string) error
}

type TokenRole string

var (
	_ TokenRepository = &tokenRepo{}
	// displayNameSanitize is used to sanitize a display name given to a token.
	displayNameSanitize = regexp.MustCompile("[^a-zA-Z0-9-]")
)

type tokenRepo struct {
	pool   *pgxpool.Pool
	logger *log.Helper
}

func NewTokenRepository(pool *pgxpool.Pool, logger log.Logger) *tokenRepo {
	return &tokenRepo{
		pool:   pool,
		logger: log.NewHelper(log.With(logger, "module", "data/TokenRepository")),
	}
}

func (r *tokenRepo) GenerateToken(ctx context.Context) string {
	return ""
}

func (r *tokenRepo) RegisterToken(ctx context.Context, token, name, parentToken string) error {
	// Insert module
	_, err := r.pool.Exec(ctx,
		"INSERT INTO tokens (token, name, role, parent_token) VALUES ($1) ON CONFLICT (name) DO NOTHING",
		token)
	if err != nil {
		return fmt.Errorf("could not insert token into database: %w", err)
	}

	return nil
}

func (r *tokenRepo) RevokeToken(ctx context.Context, token string) error {
	// Insert module
	_, err := r.pool.Exec(ctx,
		"UPDATE tokens SET is_deleted=1 WHERE token=$1",
		token)
	if err != nil {
		return fmt.Errorf("could not update token in the database: %w", err)
	}

	return nil
}
