package server

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/data"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TokenServer struct {
	v1.UnimplementedTokenServiceServer
	tokenRepository data.TokenRepository
	logger          *log.Helper
}

func NewTokenServer(
	tokenRepository data.TokenRepository,
	logger log.Logger,
) *TokenServer {
	return &TokenServer{
		tokenRepository: tokenRepository,
		logger:          log.NewHelper(log.With(logger, "module", "server/TokenServer")),
	}
}

func (TokenServer) RegisterToken(context.Context, *v1.RegisterTokenRequest) (*v1.RegisterTokenResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterToken not implemented")
}
func (TokenServer) RevokeToken(context.Context, *v1.RevokeTokenRequest) (*v1.RevokeTokenResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RevokeToken not implemented")
}
