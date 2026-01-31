package background

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
	"github.com/yoheimuta/go-protoparser/v4/parser"
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
	prevBody := previous.ProtoBody
	currBody := current.ProtoBody

	if prevBody == nil || currBody == nil {
		return false
	}

	// Check for removed messages
	prevMessages := extractMessagesFromParsed(prevBody.Messages)
	currMessages := extractMessagesFromParsed(currBody.Messages)
	for msg := range prevMessages {
		if _, exists := currMessages[msg]; !exists {
			return true
		}
	}

	// Check for removed services
	prevServices := extractServicesFromParsed(prevBody.Services)
	currServices := extractServicesFromParsed(currBody.Services)
	for svc := range prevServices {
		if _, exists := currServices[svc]; !exists {
			return true
		}
	}

	// Check for removed enums
	prevEnums := extractEnumsFromParsed(prevBody.Enums)
	currEnums := extractEnumsFromParsed(currBody.Enums)
	for enum := range prevEnums {
		if _, exists := currEnums[enum]; !exists {
			return true
		}
	}

	// Check for removed fields in messages
	prevFields := extractFieldsFromMessages(prevBody.Messages)
	currFields := extractFieldsFromMessages(currBody.Messages)
	for fieldKey, fieldNum := range prevFields {
		if currFieldNum, exists := currFields[fieldKey]; exists {
			// Field number changed - breaking
			if fieldNum != currFieldNum {
				return true
			}
		} else {
			// Field removed - breaking
			return true
		}
	}

	// Check for removed RPCs in services
	prevRPCs := extractRPCsFromParsed(prevBody.Services)
	currRPCs := extractRPCsFromParsed(currBody.Services)
	for rpc := range prevRPCs {
		if _, exists := currRPCs[rpc]; !exists {
			return true
		}
	}

	// Check for removed enum values
	prevEnumValues := extractEnumValuesFromParsed(prevBody.Enums)
	currEnumValues := extractEnumValuesFromParsed(currBody.Enums)
	for enumVal := range prevEnumValues {
		if _, exists := currEnumValues[enumVal]; !exists {
			return true
		}
	}

	return false
}

// hasPotentiallyBreakingChangesFromParsed checks for changes that might break compatibility using parsed structures
func hasPotentiallyBreakingChangesFromParsed(previous, current *unordered.Proto) bool {
	prevBody := previous.ProtoBody
	currBody := current.ProtoBody

	if prevBody == nil || currBody == nil {
		return false
	}

	// Check if required fields were added
	prevRequiredFields := countRequiredFields(prevBody.Messages)
	currRequiredFields := countRequiredFields(currBody.Messages)
	if currRequiredFields > prevRequiredFields {
		return true
	}

	// Check for field type changes
	prevFieldTypes := extractFieldTypesFromMessages(prevBody.Messages)
	currFieldTypes := extractFieldTypesFromMessages(currBody.Messages)
	for fieldKey, prevType := range prevFieldTypes {
		if currType, exists := currFieldTypes[fieldKey]; exists {
			if prevType != currType {
				return true
			}
		}
	}

	// Check for package changes
	prevPackage := extractPackageFromParsed(prevBody.Packages)
	currPackage := extractPackageFromParsed(currBody.Packages)

	return prevPackage != currPackage
}

// Helper functions to extract proto elements from parsed structures

func extractMessagesFromParsed(messages []*unordered.Message) map[string]bool {
	result := make(map[string]bool)
	for _, msg := range messages {
		result[msg.MessageName] = true
		// Also extract nested messages
		if msg.MessageBody != nil {
			for k, v := range extractMessagesFromParsed(msg.MessageBody.Messages) {
				result[msg.MessageName+"."+k] = v
			}
		}
	}
	return result
}

func extractServicesFromParsed(services []*unordered.Service) map[string]bool {
	result := make(map[string]bool)
	for _, svc := range services {
		result[svc.ServiceName] = true
	}
	return result
}

func extractEnumsFromParsed(enums []*unordered.Enum) map[string]bool {
	result := make(map[string]bool)
	for _, enum := range enums {
		result[enum.EnumName] = true
	}
	return result
}

func extractFieldsFromMessages(messages []*unordered.Message) map[string]string {
	result := make(map[string]string)
	for _, msg := range messages {
		if msg.MessageBody == nil {
			continue
		}
		// Extract regular fields
		for _, field := range msg.MessageBody.Fields {
			key := msg.MessageName + "." + field.FieldName
			result[key] = field.FieldNumber
		}
		// Extract map fields
		for _, mapField := range msg.MessageBody.Maps {
			key := msg.MessageName + "." + mapField.MapName
			result[key] = mapField.FieldNumber
		}
		// Extract oneof fields
		for _, oneof := range msg.MessageBody.Oneofs {
			for _, field := range oneof.OneofFields {
				key := msg.MessageName + "." + field.FieldName
				result[key] = field.FieldNumber
			}
		}
		// Recursively extract nested message fields
		for k, v := range extractFieldsFromMessages(msg.MessageBody.Messages) {
			result[msg.MessageName+"."+k] = v
		}
	}
	return result
}

func extractRPCsFromParsed(services []*unordered.Service) map[string]bool {
	result := make(map[string]bool)
	for _, svc := range services {
		if svc.ServiceBody == nil {
			continue
		}
		for _, rpc := range svc.ServiceBody.RPCs {
			key := svc.ServiceName + "." + rpc.RPCName
			result[key] = true
		}
	}
	return result
}

func extractEnumValuesFromParsed(enums []*unordered.Enum) map[string]bool {
	result := make(map[string]bool)
	for _, enum := range enums {
		if enum.EnumBody == nil {
			continue
		}
		for _, field := range enum.EnumBody.EnumFields {
			key := enum.EnumName + "." + field.Ident
			result[key] = true
		}
	}
	return result
}

func extractPackageFromParsed(packages []*parser.Package) string {
	if len(packages) > 0 {
		return packages[0].Name
	}
	return ""
}

func countRequiredFields(messages []*unordered.Message) int {
	count := 0
	for _, msg := range messages {
		if msg.MessageBody == nil {
			continue
		}
		for _, field := range msg.MessageBody.Fields {
			if field.IsRequired {
				count++
			}
		}
		// Recursively count in nested messages
		count += countRequiredFields(msg.MessageBody.Messages)
	}
	return count
}

func extractFieldTypesFromMessages(messages []*unordered.Message) map[string]string {
	result := make(map[string]string)
	for _, msg := range messages {
		if msg.MessageBody == nil {
			continue
		}
		for _, field := range msg.MessageBody.Fields {
			key := msg.MessageName + "." + field.FieldName
			result[key] = field.Type
		}
		for _, mapField := range msg.MessageBody.Maps {
			key := msg.MessageName + "." + mapField.MapName
			result[key] = "map<" + mapField.KeyType + "," + mapField.Type + ">"
		}
		// Recursively extract from nested messages
		for k, v := range extractFieldTypesFromMessages(msg.MessageBody.Messages) {
			result[msg.MessageName+"."+k] = v
		}
	}
	return result
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
