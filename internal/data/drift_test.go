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
	moduleEvents, err := suite.driftRepository.GetDriftEventsForModule(ctx, moduleName, "")
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
	moduleEvents, err := suite.driftRepository.GetDriftEventsForModule(ctx, moduleID, "")
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

func TestDriftRepository_GetModuleDependencyDriftStatuses(t *testing.T) {
	ctx := context.Background()

	moduleA := "test-module-a-dep-drift-" + time.Now().Format("20060102150405")
	moduleB := "test-module-b-dep-drift-" + time.Now().Format("20060102150405")

	err := suite.registryRepository.RegisterModule(ctx, moduleA)
	require.NoError(t, err)
	err = suite.registryRepository.RegisterModule(ctx, moduleB)
	require.NoError(t, err)

	// Dependency module baseline tag.
	_, err = suite.registryRepository.PushModule(ctx, moduleB, "v1.0.0", []*v1.ProtoFile{
		{Filename: "dep.proto", Content: `syntax = "proto3"; message DepV1 { string id = 1; }`},
	})
	require.NoError(t, err)
	tagBV1ID, err := suite.registryRepository.GetModuleTagId(ctx, moduleB, "v1.0.0")
	require.NoError(t, err)
	moduleBID, _, err := suite.driftRepository.GetTagInfo(ctx, tagBV1ID)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Dependency module newer tag with non-breaking drift.
	_, err = suite.registryRepository.PushModule(ctx, moduleB, "v1.1.0", []*v1.ProtoFile{
		{Filename: "dep.proto", Content: `syntax = "proto3"; message DepV1 { string id = 1; string name = 2; }`},
	})
	require.NoError(t, err)
	tagBV11ID, err := suite.registryRepository.GetModuleTagId(ctx, moduleB, "v1.1.0")
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Dependency module newer tag with breaking drift.
	_, err = suite.registryRepository.PushModule(ctx, moduleB, "v2.0.0", []*v1.ProtoFile{
		{Filename: "dep.proto", Content: `syntax = "proto3"; message DepV2 {}`},
	})
	require.NoError(t, err)
	tagBV2ID, err := suite.registryRepository.GetModuleTagId(ctx, moduleB, "v2.0.0")
	require.NoError(t, err)

	// Dependent module pinned to B@v1.0.0.
	_, err = suite.registryRepository.PushModule(ctx, moduleA, "v1.0.0", []*v1.ProtoFile{
		{Filename: "main.proto", Content: `syntax = "proto3"; message Main { string id = 1; }`},
	})
	require.NoError(t, err)
	err = suite.registryRepository.AddModuleDependencies(ctx, moduleA, "v1.0.0", []*v1.Dependency{
		{Name: moduleB, Tag: "v1.0.0"},
	})
	require.NoError(t, err)

	// Seed drift events for newer dependency tags.
	err = suite.driftRepository.SaveDriftEvents(ctx, []model.DriftEvent{
		{
			ModuleID:    moduleBID,
			TagID:       tagBV11ID,
			Filename:    "dep.proto",
			EventType:   model.DriftEventTypeModified,
			Severity:    model.DriftSeverityInfo,
			CurrentHash: "infohash",
			DetectedAt:  time.Now(),
		},
		{
			ModuleID:    moduleBID,
			TagID:       tagBV2ID,
			Filename:    "dep.proto",
			EventType:   model.DriftEventTypeModified,
			Severity:    model.DriftSeverityCritical,
			CurrentHash: "crithash",
			DetectedAt:  time.Now(),
		},
	})
	require.NoError(t, err)

	statuses, err := suite.driftRepository.GetModuleDependencyDriftStatuses(ctx, moduleA, "v1.0.0")
	require.NoError(t, err)
	require.Len(t, statuses, 2)

	assert.Equal(t, moduleB, statuses[0].DependencyName)
	assert.Equal(t, "v1.0.0", statuses[0].CurrentTag)
	assert.Equal(t, "v1.1.0", statuses[0].TargetTag)
	assert.Equal(t, model.DriftSeverityInfo, statuses[0].Severity)
	assert.Equal(t, model.DependencyDriftRecommendationSuggestUpdate, statuses[0].Recommendation)

	assert.Equal(t, moduleB, statuses[1].DependencyName)
	assert.Equal(t, "v1.0.0", statuses[1].CurrentTag)
	assert.Equal(t, "v2.0.0", statuses[1].TargetTag)
	assert.Equal(t, model.DriftSeverityCritical, statuses[1].Severity)
	assert.Equal(t, model.DependencyDriftRecommendationAlertReview, statuses[1].Recommendation)
}

func TestDriftRepository_GetTagsWithoutHashes(t *testing.T) {
	ctx := context.Background()

	// Create a test module with protofiles (protofiles start without hashes)
	moduleName := "test-tags-without-hashes-" + time.Now().Format("20060102150405")
	err := suite.registryRepository.RegisterModule(ctx, moduleName)
	require.NoError(t, err)

	_, err = suite.registryRepository.PushModule(ctx, moduleName, "v1.0.0", []*v1.ProtoFile{
		{Filename: "test.proto", Content: `syntax = "proto3"; message Test {}`},
	})
	require.NoError(t, err)

	tagID, err := suite.registryRepository.GetModuleTagId(ctx, moduleName, "v1.0.0")
	require.NoError(t, err)

	// Tag should appear in the "without hashes" list
	tagsWithoutHashes, err := suite.driftRepository.GetTagsWithoutHashes(ctx)
	require.NoError(t, err)
	assert.Contains(t, tagsWithoutHashes, tagID)

	// After computing hashes, tag should no longer appear
	err = suite.driftRepository.ComputeAndStoreHashes(ctx, tagID)
	require.NoError(t, err)

	tagsWithoutHashes, err = suite.driftRepository.GetTagsWithoutHashes(ctx)
	require.NoError(t, err)
	assert.NotContains(t, tagsWithoutHashes, tagID)
}

func TestDriftRepository_GetUnacknowledgedDriftEvents(t *testing.T) {
	ctx := context.Background()

	// Create a module with a tag
	moduleName := "test-unacked-events-" + time.Now().Format("20060102150405")
	err := suite.registryRepository.RegisterModule(ctx, moduleName)
	require.NoError(t, err)

	_, err = suite.registryRepository.PushModule(ctx, moduleName, "v1.0.0", []*v1.ProtoFile{
		{Filename: "test.proto", Content: `syntax = "proto3"; message Test {}`},
	})
	require.NoError(t, err)

	tagID, err := suite.registryRepository.GetModuleTagId(ctx, moduleName, "v1.0.0")
	require.NoError(t, err)

	moduleID, _, err := suite.driftRepository.GetTagInfo(ctx, tagID)
	require.NoError(t, err)

	// Save some drift events
	events := []model.DriftEvent{
		{
			ModuleID:    moduleID,
			TagID:       tagID,
			Filename:    "unacked-file-1.proto",
			EventType:   model.DriftEventTypeAdded,
			CurrentHash: "hash1",
			Severity:    model.DriftSeverityInfo,
			DetectedAt:  time.Now(),
		},
		{
			ModuleID:    moduleID,
			TagID:       tagID,
			Filename:    "unacked-file-2.proto",
			EventType:   model.DriftEventTypeModified,
			PreviousHash: "oldhash",
			CurrentHash:  "newhash",
			Severity:     model.DriftSeverityWarning,
			DetectedAt:   time.Now(),
		},
	}
	err = suite.driftRepository.SaveDriftEvents(ctx, events)
	require.NoError(t, err)

	// Get unacknowledged events - our events should be in the list
	unackedEvents, err := suite.driftRepository.GetUnacknowledgedDriftEvents(ctx)
	require.NoError(t, err)

	foundCount := 0
	for _, e := range unackedEvents {
		if e.TagID == tagID && (e.Filename == "unacked-file-1.proto" || e.Filename == "unacked-file-2.proto") {
			foundCount++
			assert.False(t, e.Acknowledged)
			assert.Equal(t, moduleName, e.ModuleName)
			assert.Equal(t, "v1.0.0", e.TagName)
		}
	}
	assert.Equal(t, 2, foundCount, "should find both unacknowledged events")
}

func TestDriftRepository_GetDriftEventsForModule(t *testing.T) {
	ctx := context.Background()

	// Create module with two tags
	moduleName := "test-events-for-module-" + time.Now().Format("20060102150405")
	err := suite.registryRepository.RegisterModule(ctx, moduleName)
	require.NoError(t, err)

	_, err = suite.registryRepository.PushModule(ctx, moduleName, "v1.0.0", []*v1.ProtoFile{
		{Filename: "test.proto", Content: `syntax = "proto3"; message V1 {}`},
	})
	require.NoError(t, err)

	tag1ID, err := suite.registryRepository.GetModuleTagId(ctx, moduleName, "v1.0.0")
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	_, err = suite.registryRepository.PushModule(ctx, moduleName, "v2.0.0", []*v1.ProtoFile{
		{Filename: "test.proto", Content: `syntax = "proto3"; message V2 {}`},
	})
	require.NoError(t, err)

	tag2ID, err := suite.registryRepository.GetModuleTagId(ctx, moduleName, "v2.0.0")
	require.NoError(t, err)

	moduleID, _, err := suite.driftRepository.GetTagInfo(ctx, tag1ID)
	require.NoError(t, err)

	// Save events for both tags
	err = suite.driftRepository.SaveDriftEvents(ctx, []model.DriftEvent{
		{
			ModuleID:    moduleID,
			TagID:       tag1ID,
			Filename:    "test.proto",
			EventType:   model.DriftEventTypeAdded,
			CurrentHash: "hash1",
			Severity:    model.DriftSeverityInfo,
			DetectedAt:  time.Now(),
		},
		{
			ModuleID:     moduleID,
			TagID:        tag2ID,
			Filename:     "test.proto",
			EventType:    model.DriftEventTypeModified,
			PreviousHash: "hash1",
			CurrentHash:  "hash2",
			Severity:     model.DriftSeverityWarning,
			DetectedAt:   time.Now(),
		},
	})
	require.NoError(t, err)

	// Test: get all events for module (no tag filter)
	allEvents, err := suite.driftRepository.GetDriftEventsForModule(ctx, moduleName, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(allEvents), 2)

	// Test: get events filtered by specific tag name
	tag1Events, err := suite.driftRepository.GetDriftEventsForModule(ctx, moduleName, "v1.0.0")
	require.NoError(t, err)

	for _, e := range tag1Events {
		assert.Equal(t, "v1.0.0", e.TagName, "all events should be for v1.0.0 tag")
	}

	tag2Events, err := suite.driftRepository.GetDriftEventsForModule(ctx, moduleName, "v2.0.0")
	require.NoError(t, err)

	for _, e := range tag2Events {
		assert.Equal(t, "v2.0.0", e.TagName, "all events should be for v2.0.0 tag")
	}

	// Test: events for non-existing module returns empty
	noEvents, err := suite.driftRepository.GetDriftEventsForModule(ctx, "nonexistent-module", "")
	require.NoError(t, err)
	assert.Empty(t, noEvents)
}
