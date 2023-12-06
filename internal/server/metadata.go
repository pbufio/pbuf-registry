package server

import (
	"context"
	"errors"

	"github.com/go-kratos/kratos/v2/log"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/utils"
)

type MetadataServer struct {
	v1.UnimplementedMetadataServiceServer
	registryRepository data.RegistryRepository
	metadataRepository data.MetadataRepository
	logger             *log.Helper
}

func NewMetadataServer(registryRepository data.RegistryRepository, metadataRepository data.MetadataRepository, logger log.Logger) *MetadataServer {
	return &MetadataServer{
		registryRepository: registryRepository,
		metadataRepository: metadataRepository,
		logger:             log.NewHelper(log.With(logger, "module", "server/MetadataServer")),
	}
}

func (m MetadataServer) GetMetadata(ctx context.Context, request *v1.GetMetadataRequest) (*v1.GetMetadataResponse, error) {
	if request.Name == "" {
		return nil, errors.New("module name is required")
	}

	if request.Tag == "" {
		return nil, errors.New("tag is required")
	}

	tagId, err := m.registryRepository.GetModuleTagId(ctx, request.Name, request.Tag)
	if err != nil {
		m.logger.Infof("error getting module tag id: %v", err)
		return nil, err
	}

	tagMeta, err := m.metadataRepository.GetTagMetaByTagId(ctx, tagId)
	if err != nil {
		m.logger.Infof("error getting tag meta: %v", err)
		return nil, err
	}

	if tagMeta == nil {
		return nil, errors.New("tag meta not found")
	}

	parsedProtoFiles, err := m.metadataRepository.GetParsedProtoFiles(ctx, tagId)
	if err != nil {
		m.logger.Infof("error getting metadata from DB: %v", err)
		return nil, err
	}

	return &v1.GetMetadataResponse{
		Packages: utils.ToMetadata(parsedProtoFiles),
	}, nil
}
