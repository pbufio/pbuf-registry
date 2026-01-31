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
	driftRepository    data.DriftRepository
	log                *log.Helper
}

func NewProtoParsingDaemon(metadataRepository data.MetadataRepository, driftRepository data.DriftRepository, logger log.Logger) Daemon {
	return &protoParsingDaemon{
		metadataRepository: metadataRepository,
		driftRepository:    driftRepository,
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

	// Track successfully processed tags for drift detection
	var processedTagIds []string

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

		processedTagIds = append(processedTagIds, tagId)
	}

	p.log.Infof("Proto parsing finished, processed %d tags", len(processedTagIds))

	// Run drift detection for processed tags
	if len(processedTagIds) > 0 {
		p.runDriftDetection(ctx, processedTagIds)
	}

	return nil
}

// runDriftDetection runs drift detection for the given tags
func (p protoParsingDaemon) runDriftDetection(ctx context.Context, tagIds []string) {
	p.log.Infof("Running drift detection for %d tags", len(tagIds))

	// Phase 1: Compute and store hashes for ALL tags that don't have them yet
	// This ensures previous tags have hashes before we compare against them
	tagsWithoutHashes, err := p.driftRepository.GetTagsWithoutHashes(ctx)
	if err != nil {
		p.log.Errorf("GetTagsWithoutHashes error: %v", err)
		return
	}

	p.log.Infof("Phase 1: Found %d tags without hashes", len(tagsWithoutHashes))

	for _, tagID := range tagsWithoutHashes {
		err := p.driftRepository.ComputeAndStoreHashes(ctx, tagID)
		if err != nil {
			p.log.Errorf("ComputeAndStoreHashes error for tag %s: %v", tagID, err)
			continue
		}
		p.log.Infof("Computed hashes for tag %s", tagID)
	}

	// Phase 2: Detect drift for processed tags
	p.log.Infof("Phase 2: Detecting drift for %d processed tags", len(tagIds))

	driftDetector := &driftDetection{
		driftRepository:    p.driftRepository,
		metadataRepository: p.metadataRepository,
		log:                p.log,
	}

	for _, tagID := range tagIds {
		result, err := driftDetector.detectDrift(ctx, tagID)
		if err != nil {
			p.log.Errorf("detectDrift error for tag %s: %v", tagID, err)
			continue
		}

		if result.HasDrift() {
			p.log.Infof("Drift detected for tag %s: %d added, %d modified, %d deleted",
				tagID, len(result.Added), len(result.Modified), len(result.Deleted))

			// Collect all events and save
			events := append(result.Added, result.Modified...)
			events = append(events, result.Deleted...)

			err = p.driftRepository.SaveDriftEvents(ctx, events)
			if err != nil {
				p.log.Errorf("SaveDriftEvents error for tag %s: %v", tagID, err)
				continue
			}

			p.log.Infof("Saved %d drift events for tag %s", len(events), tagID)

			// Log significant changes for visibility
			driftDetector.logSignificantChanges(result)
		} else {
			p.log.Infof("No drift detected for tag %s", tagID)
		}
	}

	p.log.Infof("Drift detection finished")
}
