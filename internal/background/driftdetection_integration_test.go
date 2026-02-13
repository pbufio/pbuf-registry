package background_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5/pgxpool"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/background"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/pbufio/pbuf-registry/migrations"
	"github.com/pbufio/pbuf-registry/test_utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Integration test suite for drift detection
type driftIntegrationSuite struct {
	psqlContainer      *test_utils.PostgreSQLContainer
	pool               *pgxpool.Pool
	registryRepository data.RegistryRepository
	metadataRepository data.MetadataRepository
	driftRepository    data.DriftRepository
}

func setupSuite(t *testing.T) *driftIntegrationSuite {
	ctx, ctxCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer ctxCancel()

	psqlContainer, err := test_utils.NewPostgreSQLContainer(ctx)
	require.NoError(t, err)

	// Wait for container to be ready
	time.Sleep(5 * time.Second)

	dsn := psqlContainer.GetDSN()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	migrations.Migrate(db)

	return &driftIntegrationSuite{
		psqlContainer:      psqlContainer,
		pool:               pool,
		registryRepository: data.NewRegistryRepository(pool, log.DefaultLogger),
		metadataRepository: data.NewMetadataRepository(pool, log.DefaultLogger),
		driftRepository:    data.NewDriftRepository(pool, log.DefaultLogger),
	}
}

func (s *driftIntegrationSuite) tearDown(t *testing.T) {
	ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ctxCancel()

	if s.pool != nil {
		s.pool.Close()
	}
	if s.psqlContainer != nil {
		err := s.psqlContainer.Terminate(ctx)
		require.NoError(t, err)
	}
}

// TestDriftDetection_EndToEnd_FullFlow tests the complete drift detection flow:
// 1. Create module with two tags
// 2. Add proto files to both tags (with changes between them)
// 3. Parse proto files and save parsed data
// 4. Run drift detection
// 5. Validate the results (added, modified, deleted files with correct severities)
func TestDriftDetection_EndToEnd_FullFlow(t *testing.T) {
	suite := setupSuite(t)
	defer suite.tearDown(t)

	ctx := context.Background()

	// Create a unique module name for this test
	moduleName := "test-drift-e2e-" + time.Now().Format("20060102150405.000")
	err := suite.registryRepository.RegisterModule(ctx, moduleName)
	require.NoError(t, err)

	// === TAG 1: Baseline version ===
	tag1Name := "v1.0.0"
	tag1Files := []*v1.ProtoFile{
		{
			Filename: "user.proto",
			Content: `syntax = "proto3";
package myapp.user;

message User {
  string id = 1;
  string name = 2;
  string email = 3;
}

message GetUserRequest {
  string id = 1;
}

message GetUserResponse {
  User user = 1;
}

service UserService {
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
  rpc DeleteUser(GetUserRequest) returns (GetUserResponse);
}
`,
		},
		{
			Filename: "common.proto",
			Content: `syntax = "proto3";
package myapp.common;

message Timestamp {
  int64 seconds = 1;
  int32 nanos = 2;
}

enum Status {
  STATUS_UNSPECIFIED = 0;
  STATUS_ACTIVE = 1;
  STATUS_INACTIVE = 2;
}
`,
		},
		{
			Filename: "to_be_deleted.proto",
			Content: `syntax = "proto3";
package myapp.deprecated;

message OldMessage {
  string value = 1;
}
`,
		},
	}

	// Push tag1
	_, err = suite.registryRepository.PushModule(ctx, moduleName, tag1Name, tag1Files)
	require.NoError(t, err)

	_, err = suite.registryRepository.GetModuleTagId(ctx, moduleName, tag1Name)
	require.NoError(t, err)

	// Small delay to ensure different timestamps
	time.Sleep(150 * time.Millisecond)

	// === TAG 2: Modified version ===
	tag2Name := "v2.0.0"
	tag2Files := []*v1.ProtoFile{
		{
			Filename: "user.proto",
			Content: `syntax = "proto3";
package myapp.user;

message User {
  string id = 1;
  string name = 2;
  string email = 3;
  string phone = 4;
}

message GetUserRequest {
  string id = 1;
}

message GetUserResponse {
  User user = 1;
}

service UserService {
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
}
`,
		},
		{
			Filename: "common.proto",
			Content: `syntax = "proto3";
package myapp.common;

message Timestamp {
  int64 seconds = 1;
  int32 nanos = 2;
}

enum Status {
  STATUS_UNSPECIFIED = 0;
  STATUS_ACTIVE = 1;
  STATUS_INACTIVE = 2;
  STATUS_PENDING = 3;
}
`,
		},
		{
			Filename: "new_feature.proto",
			Content: `syntax = "proto3";
package myapp.feature;

message NewFeature {
  string id = 1;
  string description = 2;
}
`,
		},
	}

	// Push tag2
	_, err = suite.registryRepository.PushModule(ctx, moduleName, tag2Name, tag2Files)
	require.NoError(t, err)

	tag2ID, err := suite.registryRepository.GetModuleTagId(ctx, moduleName, tag2Name)
	require.NoError(t, err)

	// === RUN DRIFT DETECTION ===
	// The proto parsing daemon will compute hashes for both tags and then detect drift
	daemon := background.NewProtoParsingDaemon(suite.metadataRepository, suite.driftRepository, log.DefaultLogger)

	// Run the daemon (it will detect drift for tags that were processed)
	err = daemon.Run()
	require.NoError(t, err)

	// === VALIDATE RESULTS ===
	events, err := suite.driftRepository.GetDriftEventsForModule(ctx, moduleName, "")
	require.NoError(t, err)

	// We should have drift events for tag2 (comparing to tag1)
	// Filter events for tag2 only
	var tag2Events []model.DriftEvent
	for _, e := range events {
		if e.TagID == tag2ID {
			tag2Events = append(tag2Events, e)
		}
	}

	// Verify we have the expected events
	t.Logf("Found %d drift events for tag2", len(tag2Events))

	// Build maps for easier assertions
	eventsByFilename := make(map[string]model.DriftEvent)
	for _, e := range tag2Events {
		eventsByFilename[e.Filename] = e
		t.Logf("Event: file=%s type=%s severity=%s", e.Filename, e.EventType, e.Severity)
	}

	// 1. new_feature.proto should be ADDED with INFO severity
	addedEvent, ok := eventsByFilename["new_feature.proto"]
	assert.True(t, ok, "new_feature.proto should have an event")
	if ok {
		assert.Equal(t, model.DriftEventTypeAdded, addedEvent.EventType)
		assert.Equal(t, model.DriftSeverityInfo, addedEvent.Severity)
	}

	// 2. to_be_deleted.proto should be DELETED with CRITICAL severity
	deletedEvent, ok := eventsByFilename["to_be_deleted.proto"]
	assert.True(t, ok, "to_be_deleted.proto should have an event")
	if ok {
		assert.Equal(t, model.DriftEventTypeDeleted, deletedEvent.EventType)
		assert.Equal(t, model.DriftSeverityCritical, deletedEvent.Severity)
	}

	// 3. user.proto should be MODIFIED (RPC removed = breaking change = CRITICAL)
	userEvent, ok := eventsByFilename["user.proto"]
	assert.True(t, ok, "user.proto should have an event")
	if ok {
		assert.Equal(t, model.DriftEventTypeModified, userEvent.EventType)
		assert.Equal(t, model.DriftSeverityCritical, userEvent.Severity, "Removing an RPC should be critical")
	}

	// 4. common.proto should be MODIFIED (enum value added = non-breaking = INFO)
	commonEvent, ok := eventsByFilename["common.proto"]
	assert.True(t, ok, "common.proto should have an event")
	if ok {
		assert.Equal(t, model.DriftEventTypeModified, commonEvent.EventType)
		assert.Equal(t, model.DriftSeverityInfo, commonEvent.Severity, "Adding an enum value should be info")
	}

	// Verify total event count
	assert.Equal(t, 4, len(tag2Events), "Should have exactly 4 drift events")
}

// TestDriftDetection_BreakingChanges tests specific breaking change detection scenarios
func TestDriftDetection_BreakingChanges(t *testing.T) {
	suite := setupSuite(t)
	defer suite.tearDown(t)

	ctx := context.Background()

	testCases := []struct {
		name             string
		previousContent  string
		currentContent   string
		expectedSeverity model.DriftSeverity
		description      string
	}{
		{
			name: "message_removed",
			previousContent: `syntax = "proto3";
package test;
message Foo { string id = 1; }
message Bar { string name = 1; }
`,
			currentContent: `syntax = "proto3";
package test;
message Foo { string id = 1; }
`,
			expectedSeverity: model.DriftSeverityCritical,
			description:      "Removing a message is a breaking change",
		},
		{
			name: "field_removed",
			previousContent: `syntax = "proto3";
package test;
message Foo {
  string id = 1;
  string name = 2;
}
`,
			currentContent: `syntax = "proto3";
package test;
message Foo {
  string id = 1;
}
`,
			expectedSeverity: model.DriftSeverityCritical,
			description:      "Removing a field is a breaking change",
		},
		{
			name: "service_removed",
			previousContent: `syntax = "proto3";
package test;
message Req { string id = 1; }
message Res { string data = 1; }
service FooService {
  rpc Get(Req) returns (Res);
}
service BarService {
  rpc Get(Req) returns (Res);
}
`,
			currentContent: `syntax = "proto3";
package test;
message Req { string id = 1; }
message Res { string data = 1; }
service FooService {
  rpc Get(Req) returns (Res);
}
`,
			expectedSeverity: model.DriftSeverityCritical,
			description:      "Removing a service is a breaking change",
		},
		{
			name: "enum_value_removed",
			previousContent: `syntax = "proto3";
package test;
enum Status {
  UNKNOWN = 0;
  ACTIVE = 1;
  INACTIVE = 2;
}
`,
			currentContent: `syntax = "proto3";
package test;
enum Status {
  UNKNOWN = 0;
  ACTIVE = 1;
}
`,
			expectedSeverity: model.DriftSeverityCritical,
			description:      "Removing an enum value is a breaking change",
		},
		{
			name: "field_type_changed",
			previousContent: `syntax = "proto3";
package test;
message Foo {
  string id = 1;
}
`,
			currentContent: `syntax = "proto3";
package test;
message Foo {
  int32 id = 1;
}
`,
			expectedSeverity: model.DriftSeverityWarning,
			description:      "Changing a field type is potentially breaking",
		},
		{
			name: "package_changed",
			previousContent: `syntax = "proto3";
package test.v1;
message Foo { string id = 1; }
`,
			currentContent: `syntax = "proto3";
package test.v2;
message Foo { string id = 1; }
`,
			expectedSeverity: model.DriftSeverityWarning,
			description:      "Changing package is potentially breaking",
		},
		{
			name: "field_added",
			previousContent: `syntax = "proto3";
package test;
message Foo {
  string id = 1;
}
`,
			currentContent: `syntax = "proto3";
package test;
message Foo {
  string id = 1;
  string name = 2;
}
`,
			expectedSeverity: model.DriftSeverityInfo,
			description:      "Adding an optional field is non-breaking",
		},
		{
			name: "message_added",
			previousContent: `syntax = "proto3";
package test;
message Foo { string id = 1; }
`,
			currentContent: `syntax = "proto3";
package test;
message Foo { string id = 1; }
message Bar { string name = 1; }
`,
			expectedSeverity: model.DriftSeverityInfo,
			description:      "Adding a message is non-breaking",
		},
		{
			name: "rpc_added",
			previousContent: `syntax = "proto3";
package test;
message Req { string id = 1; }
message Res { string data = 1; }
service FooService {
  rpc Get(Req) returns (Res);
}
`,
			currentContent: `syntax = "proto3";
package test;
message Req { string id = 1; }
message Res { string data = 1; }
service FooService {
  rpc Get(Req) returns (Res);
  rpc List(Req) returns (Res);
}
`,
			expectedSeverity: model.DriftSeverityInfo,
			description:      "Adding an RPC is non-breaking",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create unique module for this test case
			moduleName := "test-drift-breaking-" + tc.name + "-" + time.Now().Format("20060102150405.000")
			err := suite.registryRepository.RegisterModule(ctx, moduleName)
			require.NoError(t, err)

			// Create tag1 with previous content
			tag1Files := []*v1.ProtoFile{{Filename: "test.proto", Content: tc.previousContent}}
			_, err = suite.registryRepository.PushModule(ctx, moduleName, "v1.0.0", tag1Files)
			require.NoError(t, err)

			time.Sleep(100 * time.Millisecond)

			// Create tag2 with current content
			tag2Files := []*v1.ProtoFile{{Filename: "test.proto", Content: tc.currentContent}}
			_, err = suite.registryRepository.PushModule(ctx, moduleName, "v2.0.0", tag2Files)
			require.NoError(t, err)

			tag2ID, err := suite.registryRepository.GetModuleTagId(ctx, moduleName, "v2.0.0")
			require.NoError(t, err)

			// Run drift detection (daemon will parse proto files and detect drift)
			daemon := background.NewProtoParsingDaemon(suite.metadataRepository, suite.driftRepository, log.DefaultLogger)
			err = daemon.Run()
			require.NoError(t, err)

			// Get events for the module
			events, err := suite.driftRepository.GetDriftEventsForModule(ctx, moduleName, "")
			require.NoError(t, err)

			// Find the event for tag2
			var foundEvent *model.DriftEvent
			for _, e := range events {
				if e.TagID == tag2ID && e.Filename == "test.proto" {
					foundEvent = &e
					break
				}
			}

			require.NotNil(t, foundEvent, "Should have a drift event for test.proto")
			assert.Equal(t, model.DriftEventTypeModified, foundEvent.EventType)
			assert.Equal(t, tc.expectedSeverity, foundEvent.Severity, tc.description)
		})
	}
}

// TestDriftDetection_MultipleFilesScenario tests drift detection with multiple files changing
func TestDriftDetection_MultipleFilesScenario(t *testing.T) {
	suite := setupSuite(t)
	defer suite.tearDown(t)

	ctx := context.Background()

	moduleName := "test-drift-multi-" + time.Now().Format("20060102150405.000")
	err := suite.registryRepository.RegisterModule(ctx, moduleName)
	require.NoError(t, err)

	// Tag1: Multiple files
	tag1Files := []*v1.ProtoFile{
		{Filename: "a.proto", Content: `syntax = "proto3"; package a; message A { string id = 1; }`},
		{Filename: "b.proto", Content: `syntax = "proto3"; package b; message B { string id = 1; }`},
		{Filename: "c.proto", Content: `syntax = "proto3"; package c; message C { string id = 1; }`},
	}

	_, err = suite.registryRepository.PushModule(ctx, moduleName, "v1.0.0", tag1Files)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Tag2: a.proto unchanged, b.proto modified, c.proto deleted, d.proto added
	tag2Files := []*v1.ProtoFile{
		{Filename: "a.proto", Content: `syntax = "proto3"; package a; message A { string id = 1; }`},                  // unchanged
		{Filename: "b.proto", Content: `syntax = "proto3"; package b; message B { string id = 1; string name = 2; }`}, // modified (field added)
		{Filename: "d.proto", Content: `syntax = "proto3"; package d; message D { string id = 1; }`},                  // new file
	}

	_, err = suite.registryRepository.PushModule(ctx, moduleName, "v2.0.0", tag2Files)
	require.NoError(t, err)
	tag2ID, err := suite.registryRepository.GetModuleTagId(ctx, moduleName, "v2.0.0")
	require.NoError(t, err)

	// Run drift detection (daemon will parse proto files and detect drift)
	daemon := background.NewProtoParsingDaemon(suite.metadataRepository, suite.driftRepository, log.DefaultLogger)
	err = daemon.Run()
	require.NoError(t, err)

	// Get events
	events, err := suite.driftRepository.GetDriftEventsForModule(ctx, moduleName, "")
	require.NoError(t, err)

	// Filter for tag2
	var tag2Events []model.DriftEvent
	for _, e := range events {
		if e.TagID == tag2ID {
			tag2Events = append(tag2Events, e)
		}
	}

	// Should have 3 events: b.proto modified, c.proto deleted, d.proto added
	// a.proto should NOT have an event (unchanged)
	assert.Equal(t, 3, len(tag2Events), "Should have 3 drift events")

	eventsByFilename := make(map[string]model.DriftEvent)
	for _, e := range tag2Events {
		eventsByFilename[e.Filename] = e
	}

	// a.proto should NOT be present
	_, hasA := eventsByFilename["a.proto"]
	assert.False(t, hasA, "a.proto should not have an event (unchanged)")

	// b.proto should be modified with INFO severity (field added)
	bEvent, hasB := eventsByFilename["b.proto"]
	assert.True(t, hasB, "b.proto should have an event")
	if hasB {
		assert.Equal(t, model.DriftEventTypeModified, bEvent.EventType)
		assert.Equal(t, model.DriftSeverityInfo, bEvent.Severity)
	}

	// c.proto should be deleted with CRITICAL severity
	cEvent, hasC := eventsByFilename["c.proto"]
	assert.True(t, hasC, "c.proto should have an event")
	if hasC {
		assert.Equal(t, model.DriftEventTypeDeleted, cEvent.EventType)
		assert.Equal(t, model.DriftSeverityCritical, cEvent.Severity)
	}

	// d.proto should be added with INFO severity
	dEvent, hasD := eventsByFilename["d.proto"]
	assert.True(t, hasD, "d.proto should have an event")
	if hasD {
		assert.Equal(t, model.DriftEventTypeAdded, dEvent.EventType)
		assert.Equal(t, model.DriftSeverityInfo, dEvent.Severity)
	}
}
