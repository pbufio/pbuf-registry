package server

import (
	"context"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/model"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrDriftEventNotFound = kerrors.NotFound("DRIFT_EVENT_NOT_FOUND", "drift event not found")
)

// DriftServer implements the DriftService gRPC service
type DriftServer struct {
	v1.UnimplementedDriftServiceServer

	driftRepo data.DriftRepository
	logger    *log.Helper
}

// NewDriftServer creates a new DriftServer
func NewDriftServer(driftRepo data.DriftRepository, logger log.Logger) *DriftServer {
	return &DriftServer{
		driftRepo: driftRepo,
		logger:    log.NewHelper(log.With(logger, "module", "server/DriftServer")),
	}
}

// ListDriftEvents returns all unacknowledged drift events
func (s *DriftServer) ListDriftEvents(ctx context.Context, request *v1.ListDriftEventsRequest) (*v1.ListDriftEventsResponse, error) {
	if request == nil {
		return nil, ErrInvalidRequest
	}

	events, err := s.driftRepo.GetUnacknowledgedDriftEvents(ctx)
	if err != nil {
		s.logger.Errorf("failed to get unacknowledged drift events: %v", err)
		return nil, kerrors.InternalServer("DRIFT_EVENTS_FETCH_FAILED", "failed to fetch drift events")
	}

	v1Events := make([]*v1.DriftEvent, 0, len(events))
	for _, event := range events {
		v1Events = append(v1Events, toV1DriftEvent(&event))
	}

	return &v1.ListDriftEventsResponse{
		Events: v1Events,
	}, nil
}

// GetModuleDriftEvents returns drift events for a specific module
func (s *DriftServer) GetModuleDriftEvents(ctx context.Context, request *v1.GetModuleDriftEventsRequest) (*v1.GetModuleDriftEventsResponse, error) {
	if request == nil || request.ModuleName == "" {
		return nil, ErrInvalidRequest
	}

	tagName := ""
	if request.TagName != nil {
		tagName = *request.TagName
	}

	events, err := s.driftRepo.GetDriftEventsForModule(ctx, request.ModuleName, tagName)
	if err != nil {
		s.logger.Errorf("failed to get drift events for module %s: %v", request.ModuleName, err)
		return nil, kerrors.InternalServer("DRIFT_EVENTS_FETCH_FAILED", "failed to fetch drift events")
	}

	v1Events := make([]*v1.DriftEvent, 0, len(events))
	for _, event := range events {
		v1Events = append(v1Events, toV1DriftEvent(&event))
	}

	return &v1.GetModuleDriftEventsResponse{
		Events: v1Events,
	}, nil
}

// AcknowledgeDriftEvent acknowledges a drift event
func (s *DriftServer) AcknowledgeDriftEvent(ctx context.Context, request *v1.AcknowledgeDriftEventRequest) (*v1.AcknowledgeDriftEventResponse, error) {
	if request == nil || request.EventId == "" {
		return nil, ErrInvalidRequest
	}

	acknowledgedBy := request.AcknowledgedBy
	if acknowledgedBy == "" {
		acknowledgedBy = "unknown"
	}

	err := s.driftRepo.AcknowledgeDriftEvent(ctx, request.EventId, acknowledgedBy)
	if err != nil {
		s.logger.Errorf("failed to acknowledge drift event %s: %v", request.EventId, err)
		return nil, kerrors.InternalServer("DRIFT_EVENT_ACKNOWLEDGE_FAILED", "failed to acknowledge drift event")
	}

	s.logger.Infof("drift event %s acknowledged by %s", request.EventId, acknowledgedBy)

	// Return a response with the event ID (we don't refetch the event for simplicity)
	return &v1.AcknowledgeDriftEventResponse{
		Event: &v1.DriftEvent{
			Id:             request.EventId,
			Acknowledged:   true,
			AcknowledgedBy: acknowledgedBy,
		},
	}, nil
}

// toV1DriftEvent converts a model.DriftEvent to a v1.DriftEvent
func toV1DriftEvent(event *model.DriftEvent) *v1.DriftEvent {
	v1Event := &v1.DriftEvent{
		Id:             event.ID,
		ModuleName:     event.ModuleName,
		TagName:        event.TagName,
		Filename:       event.Filename,
		EventType:      driftEventTypeToV1(event.EventType),
		PreviousHash:   event.PreviousHash,
		CurrentHash:    event.CurrentHash,
		Severity:       driftSeverityToV1(event.Severity),
		DetectedAt:     timestamppb.New(event.DetectedAt),
		Acknowledged:   event.Acknowledged,
		AcknowledgedBy: event.AcknowledgedBy,
	}

	if event.AcknowledgedAt != nil {
		v1Event.AcknowledgedAt = timestamppb.New(*event.AcknowledgedAt)
	}

	return v1Event
}

// driftEventTypeToV1 converts a model.DriftEventType to a v1.DriftEventType
func driftEventTypeToV1(eventType model.DriftEventType) v1.DriftEventType {
	switch eventType {
	case model.DriftEventTypeAdded:
		return v1.DriftEventType_DRIFT_EVENT_TYPE_ADDED
	case model.DriftEventTypeModified:
		return v1.DriftEventType_DRIFT_EVENT_TYPE_MODIFIED
	case model.DriftEventTypeDeleted:
		return v1.DriftEventType_DRIFT_EVENT_TYPE_DELETED
	default:
		return v1.DriftEventType_DRIFT_EVENT_TYPE_UNSPECIFIED
	}
}

// driftSeverityToV1 converts a model.DriftSeverity to a v1.DriftSeverity
func driftSeverityToV1(severity model.DriftSeverity) v1.DriftSeverity {
	switch severity {
	case model.DriftSeverityInfo:
		return v1.DriftSeverity_DRIFT_SEVERITY_INFO
	case model.DriftSeverityWarning:
		return v1.DriftSeverity_DRIFT_SEVERITY_WARNING
	case model.DriftSeverityCritical:
		return v1.DriftSeverity_DRIFT_SEVERITY_CRITICAL
	default:
		return v1.DriftSeverity_DRIFT_SEVERITY_UNSPECIFIED
	}
}
