package server

import (
	"context"
	"errors"
	"io"
	stdlog "log"
	"net"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/mocks"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func setupDriftClient(t *testing.T, driftRepo *mocks.DriftRepository) (v1.DriftServiceClient, func()) {
	t.Helper()

	buffer := 101024 * 1024
	lis := bufconn.Listen(buffer)

	grpcServer := grpc.NewServer()
	logger := log.NewStdLogger(io.Discard)
	v1.RegisterDriftServiceServer(grpcServer, NewDriftServer(driftRepo, logger))
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			stdlog.Printf("error serving server: %v", err)
		}
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("error connecting to server: %v", err)
	}

	closer := func() {
		_ = conn.Close()
		_ = lis.Close()
		grpcServer.Stop()
	}

	return v1.NewDriftServiceClient(conn), closer
}

func TestDriftServer_NilRequest_ReturnsInvalidRequest(t *testing.T) {
	s := &DriftServer{}
	ctx := context.Background()

	t.Run("ListDriftEvents", func(t *testing.T) {
		_, err := s.ListDriftEvents(ctx, nil)
		if err != ErrInvalidRequest {
			t.Fatalf("expected ErrInvalidRequest, got %v", err)
		}
	})
	t.Run("GetModuleDriftEvents", func(t *testing.T) {
		_, err := s.GetModuleDriftEvents(ctx, nil)
		if err != ErrInvalidRequest {
			t.Fatalf("expected ErrInvalidRequest, got %v", err)
		}
	})
	t.Run("AcknowledgeDriftEvent", func(t *testing.T) {
		_, err := s.AcknowledgeDriftEvent(ctx, nil)
		if err != ErrInvalidRequest {
			t.Fatalf("expected ErrInvalidRequest, got %v", err)
		}
	})
}

func TestDriftServer_ListDriftEvents_GRPC(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		driftRepo := mocks.NewDriftRepository(t)
		client, closer := setupDriftClient(t, driftRepo)
		defer closer()

		detectedAt := time.Now().UTC().Truncate(time.Second)
		events := []model.DriftEvent{
			{
				ID:           "event-1",
				ModuleID:     "module-1",
				TagID:        "tag-1",
				Filename:     "test.proto",
				EventType:    model.DriftEventTypeModified,
				PreviousHash: "hash1",
				CurrentHash:  "hash2",
				Severity:     model.DriftSeverityWarning,
				DetectedAt:   detectedAt,
				Acknowledged: false,
			},
			{
				ID:           "event-2",
				ModuleID:     "module-1",
				TagID:        "tag-1",
				Filename:     "added.proto",
				EventType:    model.DriftEventTypeAdded,
				CurrentHash:  "hash3",
				Severity:     model.DriftSeverityInfo,
				DetectedAt:   detectedAt,
				Acknowledged: false,
			},
		}

		driftRepo.On("GetUnacknowledgedDriftEvents", mock.Anything).Return(events, nil).Once()

		resp, err := client.ListDriftEvents(ctx, &v1.ListDriftEventsRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.GetEvents()) != 2 {
			t.Fatalf("expected 2 events, got %d", len(resp.GetEvents()))
		}
		if resp.GetEvents()[0].GetId() != "event-1" {
			t.Fatalf("expected event id event-1, got %s", resp.GetEvents()[0].GetId())
		}
		if resp.GetEvents()[0].GetEventType() != v1.DriftEventType_DRIFT_EVENT_TYPE_MODIFIED {
			t.Fatalf("expected event type MODIFIED, got %v", resp.GetEvents()[0].GetEventType())
		}
		if resp.GetEvents()[1].GetEventType() != v1.DriftEventType_DRIFT_EVENT_TYPE_ADDED {
			t.Fatalf("expected event type ADDED, got %v", resp.GetEvents()[1].GetEventType())
		}
	})

	t.Run("empty list", func(t *testing.T) {
		driftRepo := mocks.NewDriftRepository(t)
		client, closer := setupDriftClient(t, driftRepo)
		defer closer()

		driftRepo.On("GetUnacknowledgedDriftEvents", mock.Anything).Return([]model.DriftEvent{}, nil).Once()

		resp, err := client.ListDriftEvents(ctx, &v1.ListDriftEventsRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.GetEvents()) != 0 {
			t.Fatalf("expected 0 events, got %d", len(resp.GetEvents()))
		}
	})

	t.Run("repo error", func(t *testing.T) {
		driftRepo := mocks.NewDriftRepository(t)
		client, closer := setupDriftClient(t, driftRepo)
		defer closer()

		driftRepo.On("GetUnacknowledgedDriftEvents", mock.Anything).Return(nil, errors.New("db")).Once()

		_, err := client.ListDriftEvents(ctx, &v1.ListDriftEventsRequest{})
		requireStatusCode(t, err, codes.Internal)
	})
}

func TestDriftServer_GetModuleDriftEvents_GRPC(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		driftRepo := mocks.NewDriftRepository(t)
		client, closer := setupDriftClient(t, driftRepo)
		defer closer()

		detectedAt := time.Now().UTC().Truncate(time.Second)
		acknowledgedAt := detectedAt.Add(time.Hour)
		events := []model.DriftEvent{
			{
				ID:             "event-1",
				ModuleID:       "module-1",
				ModuleName:     "buf.build/test/module",
				TagID:          "tag-1",
				TagName:        "v1.0.0",
				Filename:       "test.proto",
				EventType:      model.DriftEventTypeDeleted,
				PreviousHash:   "hash1",
				Severity:       model.DriftSeverityCritical,
				DetectedAt:     detectedAt,
				Acknowledged:   true,
				AcknowledgedAt: &acknowledgedAt,
				AcknowledgedBy: "admin",
			},
		}

		driftRepo.On("GetDriftEventsForModule", mock.Anything, "buf.build/test/module", "").Return(events, nil).Once()

		resp, err := client.GetModuleDriftEvents(ctx, &v1.GetModuleDriftEventsRequest{ModuleName: "buf.build/test/module"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.GetEvents()) != 1 {
			t.Fatalf("expected 1 event, got %d", len(resp.GetEvents()))
		}
		if resp.GetEvents()[0].GetModuleName() != "buf.build/test/module" {
			t.Fatalf("expected module name buf.build/test/module, got %s", resp.GetEvents()[0].GetModuleName())
		}
		if resp.GetEvents()[0].GetEventType() != v1.DriftEventType_DRIFT_EVENT_TYPE_DELETED {
			t.Fatalf("expected event type DELETED, got %v", resp.GetEvents()[0].GetEventType())
		}
		if resp.GetEvents()[0].GetSeverity() != v1.DriftSeverity_DRIFT_SEVERITY_CRITICAL {
			t.Fatalf("expected severity CRITICAL, got %v", resp.GetEvents()[0].GetSeverity())
		}
		if !resp.GetEvents()[0].GetAcknowledged() {
			t.Fatalf("expected acknowledged to be true")
		}
		if resp.GetEvents()[0].GetAcknowledgedBy() != "admin" {
			t.Fatalf("expected acknowledged by admin, got %s", resp.GetEvents()[0].GetAcknowledgedBy())
		}
		if resp.GetEvents()[0].GetAcknowledgedAt() == nil {
			t.Fatalf("expected acknowledged_at to be set")
		}
	})

	t.Run("empty module_name", func(t *testing.T) {
		driftRepo := mocks.NewDriftRepository(t)
		client, closer := setupDriftClient(t, driftRepo)
		defer closer()

		_, err := client.GetModuleDriftEvents(ctx, &v1.GetModuleDriftEventsRequest{ModuleName: ""})
		requireStatusCode(t, err, codes.InvalidArgument)
	})

	t.Run("repo error", func(t *testing.T) {
		driftRepo := mocks.NewDriftRepository(t)
		client, closer := setupDriftClient(t, driftRepo)
		defer closer()

		driftRepo.On("GetDriftEventsForModule", mock.Anything, "buf.build/test/module", "").Return(nil, errors.New("db")).Once()

		_, err := client.GetModuleDriftEvents(ctx, &v1.GetModuleDriftEventsRequest{ModuleName: "buf.build/test/module"})
		requireStatusCode(t, err, codes.Internal)
	})
}

func TestDriftServer_AcknowledgeDriftEvent_GRPC(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		driftRepo := mocks.NewDriftRepository(t)
		client, closer := setupDriftClient(t, driftRepo)
		defer closer()

		driftRepo.On("AcknowledgeDriftEvent", mock.Anything, "event-1", "admin").Return(nil).Once()

		resp, err := client.AcknowledgeDriftEvent(ctx, &v1.AcknowledgeDriftEventRequest{
			EventId:        "event-1",
			AcknowledgedBy: "admin",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetEvent().GetId() != "event-1" {
			t.Fatalf("expected event id event-1, got %s", resp.GetEvent().GetId())
		}
		if !resp.GetEvent().GetAcknowledged() {
			t.Fatalf("expected acknowledged to be true")
		}
		if resp.GetEvent().GetAcknowledgedBy() != "admin" {
			t.Fatalf("expected acknowledged by admin, got %s", resp.GetEvent().GetAcknowledgedBy())
		}
	})

	t.Run("empty acknowledged_by defaults to unknown", func(t *testing.T) {
		driftRepo := mocks.NewDriftRepository(t)
		client, closer := setupDriftClient(t, driftRepo)
		defer closer()

		driftRepo.On("AcknowledgeDriftEvent", mock.Anything, "event-1", "unknown").Return(nil).Once()

		resp, err := client.AcknowledgeDriftEvent(ctx, &v1.AcknowledgeDriftEventRequest{
			EventId:        "event-1",
			AcknowledgedBy: "",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetEvent().GetAcknowledgedBy() != "unknown" {
			t.Fatalf("expected acknowledged by unknown, got %s", resp.GetEvent().GetAcknowledgedBy())
		}
	})

	t.Run("empty event_id", func(t *testing.T) {
		driftRepo := mocks.NewDriftRepository(t)
		client, closer := setupDriftClient(t, driftRepo)
		defer closer()

		_, err := client.AcknowledgeDriftEvent(ctx, &v1.AcknowledgeDriftEventRequest{
			EventId: "",
		})
		requireStatusCode(t, err, codes.InvalidArgument)
	})

	t.Run("repo error", func(t *testing.T) {
		driftRepo := mocks.NewDriftRepository(t)
		client, closer := setupDriftClient(t, driftRepo)
		defer closer()

		driftRepo.On("AcknowledgeDriftEvent", mock.Anything, "event-1", "admin").Return(errors.New("db")).Once()

		_, err := client.AcknowledgeDriftEvent(ctx, &v1.AcknowledgeDriftEventRequest{
			EventId:        "event-1",
			AcknowledgedBy: "admin",
		})
		requireStatusCode(t, err, codes.Internal)
	})
}

func TestDriftServer_toV1DriftEvent(t *testing.T) {
	t.Run("converts all fields", func(t *testing.T) {
		detectedAt := time.Now().UTC().Truncate(time.Second)
		acknowledgedAt := detectedAt.Add(time.Hour)

		event := &model.DriftEvent{
			ID:             "event-1",
			ModuleID:       "module-1",
			ModuleName:     "buf.build/test/module",
			TagID:          "tag-1",
			TagName:        "v1.0.0",
			Filename:       "test.proto",
			EventType:      model.DriftEventTypeModified,
			PreviousHash:   "hash1",
			CurrentHash:    "hash2",
			Severity:       model.DriftSeverityWarning,
			DetectedAt:     detectedAt,
			Acknowledged:   true,
			AcknowledgedAt: &acknowledgedAt,
			AcknowledgedBy: "admin",
		}

		v1Event := toV1DriftEvent(event)

		if v1Event.GetId() != "event-1" {
			t.Fatalf("expected id event-1, got %s", v1Event.GetId())
		}
		if v1Event.GetModuleName() != "buf.build/test/module" {
			t.Fatalf("expected module_name buf.build/test/module, got %s", v1Event.GetModuleName())
		}
		if v1Event.GetTagName() != "v1.0.0" {
			t.Fatalf("expected tag_name v1.0.0, got %s", v1Event.GetTagName())
		}
		if v1Event.GetFilename() != "test.proto" {
			t.Fatalf("expected filename test.proto, got %s", v1Event.GetFilename())
		}
		if v1Event.GetEventType() != v1.DriftEventType_DRIFT_EVENT_TYPE_MODIFIED {
			t.Fatalf("expected event type MODIFIED, got %v", v1Event.GetEventType())
		}
		if v1Event.GetPreviousHash() != "hash1" {
			t.Fatalf("expected previous_hash hash1, got %s", v1Event.GetPreviousHash())
		}
		if v1Event.GetCurrentHash() != "hash2" {
			t.Fatalf("expected current_hash hash2, got %s", v1Event.GetCurrentHash())
		}
		if v1Event.GetSeverity() != v1.DriftSeverity_DRIFT_SEVERITY_WARNING {
			t.Fatalf("expected severity WARNING, got %v", v1Event.GetSeverity())
		}
		if !v1Event.GetAcknowledged() {
			t.Fatalf("expected acknowledged to be true")
		}
		if v1Event.GetAcknowledgedBy() != "admin" {
			t.Fatalf("expected acknowledged_by admin, got %s", v1Event.GetAcknowledgedBy())
		}
		if v1Event.GetAcknowledgedAt() == nil {
			t.Fatalf("expected acknowledged_at to be set")
		}
	})

	t.Run("nil acknowledged_at", func(t *testing.T) {
		event := &model.DriftEvent{
			ID:             "event-1",
			ModuleID:       "module-1",
			ModuleName:     "buf.build/test/module",
			TagID:          "tag-1",
			TagName:        "v1.0.0",
			Filename:       "test.proto",
			EventType:      model.DriftEventTypeAdded,
			CurrentHash:    "hash1",
			Severity:       model.DriftSeverityInfo,
			DetectedAt:     time.Now(),
			Acknowledged:   false,
			AcknowledgedAt: nil,
		}

		v1Event := toV1DriftEvent(event)

		if v1Event.GetAcknowledgedAt() != nil {
			t.Fatalf("expected acknowledged_at to be nil")
		}
	})
}

func TestDriftServer_driftEventTypeToV1(t *testing.T) {
	tests := []struct {
		input    model.DriftEventType
		expected v1.DriftEventType
	}{
		{model.DriftEventTypeAdded, v1.DriftEventType_DRIFT_EVENT_TYPE_ADDED},
		{model.DriftEventTypeModified, v1.DriftEventType_DRIFT_EVENT_TYPE_MODIFIED},
		{model.DriftEventTypeDeleted, v1.DriftEventType_DRIFT_EVENT_TYPE_DELETED},
		{"unknown", v1.DriftEventType_DRIFT_EVENT_TYPE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := driftEventTypeToV1(tt.input)
			if result != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDriftServer_driftSeverityToV1(t *testing.T) {
	tests := []struct {
		input    model.DriftSeverity
		expected v1.DriftSeverity
	}{
		{model.DriftSeverityInfo, v1.DriftSeverity_DRIFT_SEVERITY_INFO},
		{model.DriftSeverityWarning, v1.DriftSeverity_DRIFT_SEVERITY_WARNING},
		{model.DriftSeverityCritical, v1.DriftSeverity_DRIFT_SEVERITY_CRITICAL},
		{"unknown", v1.DriftSeverity_DRIFT_SEVERITY_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := driftSeverityToV1(tt.input)
			if result != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
