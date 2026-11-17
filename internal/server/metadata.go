package server

import (
	"context"

	kerrors "github.com/go-kratos/kratos/v2/errors"
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
	if request == nil || request.Name == "" {
		return nil, kerrors.BadRequest("INVALID_ARGUMENT", "module name is required")
	}

	tag := request.Tag
	if tag == "" {
		module, err := m.registryRepository.GetModuleWithDrafts(ctx, request.Name, false)
		if err != nil {
			m.logger.Infof("error getting module: %v", err)
			return nil, err
		}
		if module == nil {
			return nil, kerrors.NotFound("MODULE_NOT_FOUND", "module not found")
		}
		if len(module.Tags) == 0 {
			return nil, kerrors.NotFound("MODULE_TAG_NOT_FOUND", "module tag not found")
		}
		tag = module.Tags[0]
	}

	tagId, err := m.registryRepository.GetModuleTagId(ctx, request.Name, tag)
	if err != nil {
		m.logger.Infof("error getting module tag id: %v", err)
		return nil, err
	}

	if tagId == "" {
		return nil, kerrors.NotFound("MODULE_TAG_NOT_FOUND", "module tag not found")
	}

	tagMeta, err := m.metadataRepository.GetTagMetaByTagId(ctx, tagId)
	if err != nil {
		m.logger.Infof("error getting tag meta: %v", err)
		return nil, err
	}

	if tagMeta == nil {
		return nil, kerrors.NotFound("TAG_META_NOT_FOUND", "tag meta not found")
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
