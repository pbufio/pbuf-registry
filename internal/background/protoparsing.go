package background

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/utils"
)

const (
	protoParsingDaemonName = "proto parsing"
)

type protoParsingDaemon struct {
	metadataRepository data.MetadataRepository
	log                *log.Helper
}

func NewProtoParsingDaemon(metadataRepository data.MetadataRepository, logger log.Logger) Daemon {
	return &protoParsingDaemon{
		metadataRepository: metadataRepository,
		log:                log.NewHelper(log.With(logger, "module", "background/ProtoParsingDaemon")),
	}
}

func (p protoParsingDaemon) Name() string {
	return protoParsingDaemonName
}

func (p protoParsingDaemon) Run() error {
	p.log.Infof("Running proto parsing")

	ctx := context.Background()

	// fetch tags that has not been processed yet
	tagIds, err := p.metadataRepository.GetUnprocessedTagIds(ctx)
	if err != nil {
		p.log.Errorf("GetUnprocessedTags error: %v", err)
		return err
	}

	// iterate over tags and parse proto files
	for _, tagId := range tagIds {
		// get all proto files for tag
		protofiles, err := p.metadataRepository.GetProtoFilesForTagId(ctx, tagId)
		if err != nil {
			p.log.Errorf("GetProtoFilesForTagId error: %v", err)
			continue
		}
		// parse proto files
		parsedProtoFiles, err := utils.ParseProtoFilesContents(protofiles)
		if err != nil {
			p.log.Errorf("ParseProtoFilesContents error: %v", err)
			return err
		}

		// save parsed proto files to database
		err = p.metadataRepository.SaveParsedProtoFiles(ctx, tagId, parsedProtoFiles)
		if err != nil {
			p.log.Errorf("SaveParsedProtoFiles error: %v", err)
			return err
		}
	}

	p.log.Infof("Proto parsing finished")
	return nil
}
