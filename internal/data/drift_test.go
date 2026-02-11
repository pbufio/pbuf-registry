package data

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDriftRepository_ComputeAndStoreHashes(t *testing.T) {
	ctx := context.Background()

	// Create a test module
	moduleName := "test-module-drift-hash-" + time.Now().Format("20060102150405")
	err := suite.registryRepository.RegisterModule(ctx, moduleName)
	require.NoError(t, err)

	// Create a tag with protofiles
	tagName := "v1.0.0"
	protoContent := `syntax = "proto3"; message TestMessage { string name = 1; }`
	protofiles := []*v1.ProtoFile{
		{Filename: "test.proto", Content: protoContent},
	}

	module, err := suite.registryRepository.PushModule(ctx, moduleName, tagName, protofiles)
	require.NoError(t, err)
	require.NotNil(t, module)

	tagID, err := suite.registryRepository.GetModuleTagId(ctx, moduleName, tagName)
	require.NoError(t, err)

	// Verify tag is in "without hashes" list
	tagsWithoutHashes, err := suite.driftRepository.GetTagsWithoutHashes(ctx)
	require.NoError(t, err)
	assert.Contains(t, tagsWithoutHashes, tagID)

	// Compute and store hashes
	err = suite.driftRepository.ComputeAndStoreHashes(ctx, tagID)
	require.NoError(t, err)

	// Verify tag is no longer in "without hashes" list
	tagsWithoutHashes, err = suite.driftRepository.GetTagsWithoutHashes(ctx)
	require.NoError(t, err)
	assert.NotContains(t, tagsWithoutHashes, tagID)

	// Verify hash was computed correctly
	fileHashes, err := suite.driftRepository.GetFileHashesForTag(ctx, tagID)
	require.NoError(t, err)
	assert.Len(t, fileHashes, 1)

	expectedHash := sha256.Sum256([]byte(protoContent))
	expectedHashStr := hex.EncodeToString(expectedHash[:])
	assert.Equal(t, expectedHashStr, fileHashes["test.proto"])
}

func TestDriftRepository_GetPreviousTagID(t *testing.T) {
	ctx := context.Background()

	// Create a test module
	moduleName := "test-module-prev-tag-" + time.Now().Format("20060102150405")
	err := suite.registryRepository.RegisterModule(ctx, moduleName)
	require.NoError(t, err)

	// Create first tag
	_, err = suite.registryRepository.PushModule(ctx, moduleName, "v1.0.0", []*v1.ProtoFile{
		{Filename: "test.proto", Content: `syntax = "proto3"; message V1 {}`},
	})
	require.NoError(t, err)

	tag1ID, err := suite.registryRepository.GetModuleTagId(ctx, moduleName, "v1.0.0")
	require.NoError(t, err)

	// Small delay to ensure different timestamps
	time.Sleep(100 * time.Millisecond)

	// Create second tag
	_, err = suite.registryRepository.PushModule(ctx, moduleName, "v2.0.0", []*v1.ProtoFile{
		{Filename: "test.proto", Content: `syntax = "proto3"; message V2 {}`},
	})
	require.NoError(t, err)

	tag2ID, err := suite.registryRepository.GetModuleTagId(ctx, moduleName, "v2.0.0")
	require.NoError(t, err)

	// Get module ID
	moduleID, _, err := suite.driftRepository.GetTagInfo(ctx, tag1ID)
	require.NoError(t, err)

	// Test: previous tag of tag2 should be tag1
	previousTagID, err := suite.driftRepository.GetPreviousTagID(ctx, moduleID, tag2ID)
	require.NoError(t, err)
	assert.Equal(t, tag1ID, previousTagID)

	// Test: previous tag of tag1 should be empty (first tag)
	previousTagID, err = suite.driftRepository.GetPreviousTagID(ctx, moduleID, tag1ID)
	require.NoError(t, err)
	assert.Empty(t, previousTagID)
}

func TestDriftRepository_SaveDriftEvents_Idempotent(t *testing.T) {
	ctx := context.Background()

	// Create a test module
	moduleName := "test-module-idempotent-" + time.Now().Format("20060102150405")
	err := suite.registryRepository.RegisterModule(ctx, moduleName)
	require.NoError(t, err)

	// Create a tag
	_, err = suite.registryRepository.PushModule(ctx, moduleName, "v1.0.0", []*v1.ProtoFile{
		{Filename: "test.proto", Content: `syntax = "proto3"; message Test {}`},
	})
	require.NoError(t, err)

	tagID, err := suite.registryRepository.GetModuleTagId(ctx, moduleName, "v1.0.0")
	require.NoError(t, err)

	moduleID, _, err := suite.driftRepository.GetTagInfo(ctx, tagID)
	require.NoError(t, err)

	// Create drift events
	events := []model.DriftEvent{
		{
			ModuleID:     moduleID,
			TagID:        tagID,
			Filename:     "test.proto",
			EventType:    model.DriftEventTypeModified,
			PreviousHash: "abc123",
			CurrentHash:  "def456",
			Severity:     model.DriftSeverityWarning,
			DetectedAt:   time.Now(),
		},
	}

	// Save events first time
	err = suite.driftRepository.SaveDriftEvents(ctx, events)
	require.NoError(t, err)

	// Save same events second time (should not error due to ON CONFLICT DO NOTHING)
	err = suite.driftRepository.SaveDriftEvents(ctx, events)
	require.NoError(t, err)

	// Verify only one event was created (idempotent)
	moduleEvents, err := suite.driftRepository.GetDriftEventsForModule(ctx, moduleID)
	require.NoError(t, err)

	// Count events with matching criteria
	matchingEvents := 0
	for _, e := range moduleEvents {
		if e.Filename == "test.proto" && e.EventType == model.DriftEventTypeModified {
			matchingEvents++
		}
	}
	assert.Equal(t, 1, matchingEvents, "should have exactly one event due to idempotency")
}

func TestDriftRepository_GetFileHashesForTag(t *testing.T) {
	ctx := context.Background()

	// Create a test module
	moduleName := "test-module-file-hashes-" + time.Now().Format("20060102150405")
	err := suite.registryRepository.RegisterModule(ctx, moduleName)
	require.NoError(t, err)

	// Create a tag with multiple protofiles
	protofiles := []*v1.ProtoFile{
		{Filename: "a.proto", Content: `syntax = "proto3"; message A {}`},
		{Filename: "b.proto", Content: `syntax = "proto3"; message B {}`},
		{Filename: "c.proto", Content: `syntax = "proto3"; message C {}`},
	}

	_, err = suite.registryRepository.PushModule(ctx, moduleName, "v1.0.0", protofiles)
	require.NoError(t, err)

	tagID, err := suite.registryRepository.GetModuleTagId(ctx, moduleName, "v1.0.0")
	require.NoError(t, err)

	// Compute hashes
	err = suite.driftRepository.ComputeAndStoreHashes(ctx, tagID)
	require.NoError(t, err)

	// Get file hashes
	fileHashes, err := suite.driftRepository.GetFileHashesForTag(ctx, tagID)
	require.NoError(t, err)
	assert.Len(t, fileHashes, 3)

	// Verify each hash
	for _, pf := range protofiles {
		expectedHash := sha256.Sum256([]byte(pf.Content))
		expectedHashStr := hex.EncodeToString(expectedHash[:])
		assert.Equal(t, expectedHashStr, fileHashes[pf.Filename], "hash mismatch for %s", pf.Filename)
	}
}

func TestDriftRepository_GetTagInfo(t *testing.T) {
	ctx := context.Background()

	// Create a test module
	moduleName := "test-module-tag-info-" + time.Now().Format("20060102150405")
	err := suite.registryRepository.RegisterModule(ctx, moduleName)
	require.NoError(t, err)

	// Create a tag
	tagName := "v1.2.3"
	module, err := suite.registryRepository.PushModule(ctx, moduleName, tagName, []*v1.ProtoFile{
		{Filename: "test.proto", Content: `syntax = "proto3"; message Test {}`},
	})
	require.NoError(t, err)

	tagID, err := suite.registryRepository.GetModuleTagId(ctx, moduleName, tagName)
	require.NoError(t, err)

	// Get tag info
	moduleID, returnedTagName, err := suite.driftRepository.GetTagInfo(ctx, tagID)
	require.NoError(t, err)
	assert.Equal(t, module.Id, moduleID)
	assert.Equal(t, tagName, returnedTagName)
}

func TestDriftRepository_GetProtoFileContents(t *testing.T) {
	ctx := context.Background()

	// Create a test module
	moduleName := "test-module-contents-" + time.Now().Format("20060102150405")
	err := suite.registryRepository.RegisterModule(ctx, moduleName)
	require.NoError(t, err)

	// Create a tag with protofiles
	protofiles := []*v1.ProtoFile{
		{Filename: "service.proto", Content: `syntax = "proto3"; service MyService {}`},
		{Filename: "types.proto", Content: `syntax = "proto3"; message MyType { int32 id = 1; }`},
	}

	_, err = suite.registryRepository.PushModule(ctx, moduleName, "v1.0.0", protofiles)
	require.NoError(t, err)

	tagID, err := suite.registryRepository.GetModuleTagId(ctx, moduleName, "v1.0.0")
	require.NoError(t, err)

	// Get proto file contents
	contents, err := suite.driftRepository.GetProtoFileContents(ctx, tagID)
	require.NoError(t, err)
	assert.Len(t, contents, 2)

	for _, pf := range protofiles {
		assert.Equal(t, pf.Content, contents[pf.Filename], "content mismatch for %s", pf.Filename)
	}
}

func TestDriftRepository_AcknowledgeDriftEvent(t *testing.T) {
	ctx := context.Background()

	// Create a test module
	moduleName := "test-module-ack-" + time.Now().Format("20060102150405")
	err := suite.registryRepository.RegisterModule(ctx, moduleName)
	require.NoError(t, err)

	// Create a tag
	_, err = suite.registryRepository.PushModule(ctx, moduleName, "v1.0.0", []*v1.ProtoFile{
		{Filename: "test.proto", Content: `syntax = "proto3"; message Test {}`},
	})
	require.NoError(t, err)

	tagID, err := suite.registryRepository.GetModuleTagId(ctx, moduleName, "v1.0.0")
	require.NoError(t, err)

	moduleID, _, err := suite.driftRepository.GetTagInfo(ctx, tagID)
	require.NoError(t, err)

	// Create and save a drift event
	events := []model.DriftEvent{
		{
			ModuleID:    moduleID,
			TagID:       tagID,
			Filename:    "ack-test.proto",
			EventType:   model.DriftEventTypeAdded,
			CurrentHash: "newhash123",
			Severity:    model.DriftSeverityInfo,
			DetectedAt:  time.Now(),
		},
	}

	err = suite.driftRepository.SaveDriftEvents(ctx, events)
	require.NoError(t, err)

	// Get unacknowledged events
	unackedEvents, err := suite.driftRepository.GetUnacknowledgedDriftEvents(ctx)
	require.NoError(t, err)

	// Find our event
	var eventID string
	for _, e := range unackedEvents {
		if e.Filename == "ack-test.proto" && e.TagID == tagID {
			eventID = e.ID
			break
		}
	}
	require.NotEmpty(t, eventID, "should find unacknowledged event")

	// Acknowledge the event
	err = suite.driftRepository.AcknowledgeDriftEvent(ctx, eventID, "test-user")
	require.NoError(t, err)

	// Verify event is no longer unacknowledged
	unackedEvents, err = suite.driftRepository.GetUnacknowledgedDriftEvents(ctx)
	require.NoError(t, err)

	found := false
	for _, e := range unackedEvents {
		if e.ID == eventID {
			found = true
			break
		}
	}
	assert.False(t, found, "acknowledged event should not be in unacknowledged list")

	// Verify event details in module events
	moduleEvents, err := suite.driftRepository.GetDriftEventsForModule(ctx, moduleID)
	require.NoError(t, err)

	for _, e := range moduleEvents {
		if e.ID == eventID {
			assert.True(t, e.Acknowledged)
			assert.Equal(t, "test-user", e.AcknowledgedBy)
			assert.NotNil(t, e.AcknowledgedAt)
			break
		}
	}
}
