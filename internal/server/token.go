package server

import (
	"github.com/go-kratos/kratos/v2/log"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/data"
)

type TokenServer struct {
	v1.UnimplementedTokenServiceServer
	tokenRepository    data.TokenRepository
	metadataRepository data.MetadataRepository
	logger             *log.Helper
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
