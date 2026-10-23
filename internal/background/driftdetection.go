package background

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/pbufio/pbuf-registry/internal/utils"
	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
)

// driftDetection contains drift detection logic used by protoParsingDaemon
type driftDetection struct {
	driftRepository    data.DriftRepository
	metadataRepository data.MetadataRepository
	log                *log.Helper
}

// detectDrift compares current file hashes with previous version and returns drift events
func (d *driftDetection) detectDrift(ctx context.Context, tagID string) (*model.DriftDetectionResult, error) {
	// Get tag info
	moduleID, _, err := d.driftRepository.GetTagInfo(ctx, tagID)
	if err != nil {
		return nil, err
	}

	result := &model.DriftDetectionResult{
		TagID:    tagID,
		ModuleID: moduleID,
		Added:    []model.DriftEvent{},
		Modified: []model.DriftEvent{},
		Deleted:  []model.DriftEvent{},
	}

	// Get previous tag
	previousTagID, err := d.driftRepository.GetPreviousTagID(ctx, moduleID, tagID)
	if err != nil {
		return nil, err
	}
	if previousTagID == "" {
		// No previous tag, this is the first version - no drift to detect
		d.log.Infof("no previous tag found for module %s, skipping drift detection", moduleID)
		return result, nil
	}

	// Get current and previous file hashes
	currentFiles, err := d.driftRepository.GetFileHashesForTag(ctx, tagID)
	if err != nil {
		return nil, err
	}

	previousFiles, err := d.driftRepository.GetFileHashesForTag(ctx, previousTagID)
	if err != nil {
		return nil, err
	}

	// Get parsed proto files for severity analysis on modified files
	var currentParsed, previousParsed map[string]*unordered.Proto

	now := time.Now()

	// Detect added and modified files
	for filename, currentHash := range currentFiles {
		previousHash, exists := previousFiles[filename]
		if !exists {
			// File was added
			result.Added = append(result.Added, model.DriftEvent{
				ModuleID:    moduleID,
				TagID:       tagID,
				Filename:    filename,
				EventType:   model.DriftEventTypeAdded,
				CurrentHash: currentHash,
				Severity:    model.DriftSeverityInfo,
				DetectedAt:  now,
			})
		} else if currentHash != previousHash {
			// File was modified - need to analyze for breaking changes
			// Lazy load parsed files only when we have modifications
			if currentParsed == nil {
				currentParsed, err = d.getParsedProtoMap(ctx, tagID)
				if err != nil {
					d.log.Errorf("error getting current parsed files: %v", err)
					currentParsed = make(map[string]*unordered.Proto)
				}
			}
			if previousParsed == nil {
				previousParsed, err = d.getParsedProtoMap(ctx, previousTagID)
				if err != nil {
					d.log.Errorf("error getting previous parsed files: %v", err)
					previousParsed = make(map[string]*unordered.Proto)
				}
			}

			severity := determineSeverityFromParsed(previousParsed[filename], currentParsed[filename])

			result.Modified = append(result.Modified, model.DriftEvent{
				ModuleID:     moduleID,
				TagID:        tagID,
				Filename:     filename,
				EventType:    model.DriftEventTypeModified,
				PreviousHash: previousHash,
				CurrentHash:  currentHash,
				Severity:     severity,
				DetectedAt:   now,
			})
		}
	}

	// Detect deleted files
	for filename, previousHash := range previousFiles {
		if _, exists := currentFiles[filename]; !exists {
			result.Deleted = append(result.Deleted, model.DriftEvent{
				ModuleID:     moduleID,
				TagID:        tagID,
				Filename:     filename,
				EventType:    model.DriftEventTypeDeleted,
				PreviousHash: previousHash,
				Severity:     model.DriftSeverityCritical, // Deleting files is always critical
				DetectedAt:   now,
			})
		}
	}

	return result, nil
}

// getParsedProtoMap retrieves parsed proto files and returns them as a map by filename
func (d *driftDetection) getParsedProtoMap(ctx context.Context, tagID string) (map[string]*unordered.Proto, error) {
	parsedFiles, err := d.metadataRepository.GetParsedProtoFiles(ctx, tagID)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*unordered.Proto)
	for _, pf := range parsedFiles {
		result[pf.Filename] = pf.Proto
	}
	return result, nil
}

// determineSeverityFromParsed analyzes the diff between two parsed proto files and determines severity
// based on backward compatibility rules
func determineSeverityFromParsed(previous, current *unordered.Proto) model.DriftSeverity {
	if previous == nil || current == nil {
		return model.DriftSeverityWarning
	}

	// Check for breaking changes (critical severity)
	if hasBreakingChangesFromParsed(previous, current) {
		return model.DriftSeverityCritical
	}

	// Check for potentially breaking changes (warning severity)
	if hasPotentiallyBreakingChangesFromParsed(previous, current) {
		return model.DriftSeverityWarning
	}

	// Non-breaking changes (info severity)
	return model.DriftSeverityInfo
}

// hasBreakingChangesFromParsed checks for changes that break backward compatibility using parsed structures
func hasBreakingChangesFromParsed(previous, current *unordered.Proto) bool {
	if previous == nil || current == nil || previous.ProtoBody == nil || current.ProtoBody == nil {
		return false
	}
	for _, item := range utils.DiffParsedProtoFile("", previous, current) {
		if item.Severity == "CRITICAL" {
			return true
		}
	}
	return false
}

// hasPotentiallyBreakingChangesFromParsed checks for changes that might break compatibility using parsed structures
func hasPotentiallyBreakingChangesFromParsed(previous, current *unordered.Proto) bool {
	if previous == nil || current == nil || previous.ProtoBody == nil || current.ProtoBody == nil {
		return false
	}
	for _, item := range utils.DiffParsedProtoFile("", previous, current) {
		if item.Severity == "WARNING" {
			return true
		}
	}
	return false
}

// logSignificantChanges logs warning-level messages for significant drift events
func (d *driftDetection) logSignificantChanges(result *model.DriftDetectionResult) {
	for _, event := range result.Modified {
		if event.Severity == model.DriftSeverityWarning || event.Severity == model.DriftSeverityCritical {
			d.log.Warnf("DRIFT ALERT: File %s was modified in module %s (tag %s), severity: %s",
				event.Filename, event.ModuleID, event.TagID, event.Severity)
		}
	}

	for _, event := range result.Deleted {
		d.log.Warnf("DRIFT ALERT: File %s was deleted in module %s (tag %s)",
			event.Filename, event.ModuleID, event.TagID)
	}
}
