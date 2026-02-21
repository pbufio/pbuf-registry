package background

import (
	"context"
	"strconv"
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

	// Package changes alter fully-qualified type references and are breaking.
	prevPackage := extractPackageFromParsed(prevBody.Packages)
	currPackage := extractPackageFromParsed(currBody.Packages)
	if prevPackage != currPackage {
		return true
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

	// Check for removed fields in messages and type changes on the same field number
	prevFields := extractFieldsByNumberFromMessages(prevBody.Messages)
	currFields := extractFieldsByNumberFromMessages(currBody.Messages)
	for fieldKey, prevField := range prevFields {
		currField, exists := currFields[fieldKey]
		if !exists {
			// Field removed (or number changed) - breaking
			return true
		}
		if prevField.Type != currField.Type {
			// Same field number with different wire type - breaking
			return true
		}
	}

	// Check for removed RPCs in services and signature changes
	prevRPCs := extractRPCDetailsFromParsed(prevBody.Services)
	currRPCs := extractRPCDetailsFromParsed(currBody.Services)
	for rpcKey, prevRPC := range prevRPCs {
		currRPC, exists := currRPCs[rpcKey]
		if !exists {
			return true
		}
		if prevRPC.RequestType != currRPC.RequestType ||
			prevRPC.ResponseType != currRPC.ResponseType ||
			prevRPC.RequestStream != currRPC.RequestStream ||
			prevRPC.ResponseStream != currRPC.ResponseStream {
			return true
		}
	}

	// Check for removed enum value numbers
	prevEnumValues := extractEnumValuesByNumberFromParsed(prevBody.Enums)
	currEnumValues := extractEnumValuesByNumberFromParsed(currBody.Enums)
	for enumValueKey := range prevEnumValues {
		if _, exists := currEnumValues[enumValueKey]; !exists {
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

	// Field rename on the same number can break generated code but keeps wire compatibility.
	prevFields := extractFieldsByNumberFromMessages(prevBody.Messages)
	currFields := extractFieldsByNumberFromMessages(currBody.Messages)
	for fieldKey, prevField := range prevFields {
		if currField, exists := currFields[fieldKey]; exists && prevField.Name != currField.Name {
			return true
		}
	}

	return false
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

type parsedField struct {
	Name string
	Type string
}

func extractFieldsByNumberFromMessages(messages []*unordered.Message) map[string]parsedField {
	result := make(map[string]parsedField)
	extractFieldsByNumberFromMessagesWithPrefix(messages, "", result)
	return result
}

func extractFieldsByNumberFromMessagesWithPrefix(messages []*unordered.Message, prefix string, result map[string]parsedField) {
	for _, msg := range messages {
		messageName := msg.MessageName
		if prefix != "" {
			messageName = prefix + "." + msg.MessageName
		}

		if msg.MessageBody == nil {
			continue
		}

		for _, field := range msg.MessageBody.Fields {
			fieldNum, ok := parseProtoNumber(field.FieldNumber)
			if !ok {
				continue
			}
			result[fieldKey(messageName, fieldNum)] = parsedField{
				Name: field.FieldName,
				Type: field.Type,
			}
		}

		for _, mapField := range msg.MessageBody.Maps {
			fieldNum, ok := parseProtoNumber(mapField.FieldNumber)
			if !ok {
				continue
			}
			result[fieldKey(messageName, fieldNum)] = parsedField{
				Name: mapField.MapName,
				Type: "map<" + mapField.KeyType + "," + mapField.Type + ">",
			}
		}

		for _, oneof := range msg.MessageBody.Oneofs {
			for _, oneofField := range oneof.OneofFields {
				fieldNum, ok := parseProtoNumber(oneofField.FieldNumber)
				if !ok {
					continue
				}
				result[fieldKey(messageName, fieldNum)] = parsedField{
					Name: oneofField.FieldName,
					Type: oneofField.Type,
				}
			}
		}

		extractFieldsByNumberFromMessagesWithPrefix(msg.MessageBody.Messages, messageName, result)
	}
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

type parsedRPC struct {
	RequestType    string
	ResponseType   string
	RequestStream  bool
	ResponseStream bool
}

func extractRPCDetailsFromParsed(services []*unordered.Service) map[string]parsedRPC {
	result := make(map[string]parsedRPC)
	for _, svc := range services {
		if svc.ServiceBody == nil {
			continue
		}
		for _, rpc := range svc.ServiceBody.RPCs {
			requestType := ""
			requestStream := false
			if rpc.RPCRequest != nil {
				requestType = rpc.RPCRequest.MessageType
				requestStream = rpc.RPCRequest.IsStream
			}
			responseType := ""
			responseStream := false
			if rpc.RPCResponse != nil {
				responseType = rpc.RPCResponse.MessageType
				responseStream = rpc.RPCResponse.IsStream
			}
			result[svc.ServiceName+"."+rpc.RPCName] = parsedRPC{
				RequestType:    requestType,
				ResponseType:   responseType,
				RequestStream:  requestStream,
				ResponseStream: responseStream,
			}
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

type parsedEnumValue struct {
	Name string
}

func extractEnumValuesByNumberFromParsed(enums []*unordered.Enum) map[string]parsedEnumValue {
	result := make(map[string]parsedEnumValue)
	for _, enum := range enums {
		if enum.EnumBody == nil {
			continue
		}
		for _, field := range enum.EnumBody.EnumFields {
			enumValueNum, ok := parseProtoNumber(field.Number)
			if !ok {
				continue
			}
			result[enumValueKey(enum.EnumName, enumValueNum)] = parsedEnumValue{
				Name: field.Ident,
			}
		}
	}
	return result
}

func parseProtoNumber(number string) (int32, bool) {
	parsed, err := strconv.ParseInt(number, 10, 32)
	if err != nil {
		return 0, false
	}
	return int32(parsed), true
}

func fieldKey(messageName string, fieldNumber int32) string {
	return messageName + "#" + strconv.FormatInt(int64(fieldNumber), 10)
}

func enumValueKey(enumName string, enumValueNumber int32) string {
	return enumName + "#" + strconv.FormatInt(int64(enumValueNumber), 10)
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
