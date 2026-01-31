package model

import "time"

// DriftEventType represents the type of drift event
type DriftEventType string

const (
	DriftEventTypeAdded    DriftEventType = "added"
	DriftEventTypeModified DriftEventType = "modified"
	DriftEventTypeDeleted  DriftEventType = "deleted"
)

// DriftSeverity represents the severity level of a drift event
type DriftSeverity string

const (
	DriftSeverityInfo     DriftSeverity = "info"
	DriftSeverityWarning  DriftSeverity = "warning"
	DriftSeverityCritical DriftSeverity = "critical"
)

// DriftEvent represents a detected change in a proto file
type DriftEvent struct {
	ID             string
	ModuleID       string
	TagID          string
	Filename       string
	EventType      DriftEventType
	PreviousHash   string
	CurrentHash    string
	Severity       DriftSeverity
	DetectedAt     time.Time
	Acknowledged   bool
	AcknowledgedAt *time.Time
	AcknowledgedBy string
}

// ProtoFileHash represents a proto file with its content hash
type ProtoFileHash struct {
	ID          string
	TagID       string
	Filename    string
	ContentHash string
}

// DriftDetectionResult holds the results of drift detection for a tag
type DriftDetectionResult struct {
	TagID    string
	ModuleID string
	Added    []DriftEvent
	Modified []DriftEvent
	Deleted  []DriftEvent
}

// HasDrift returns true if any drift was detected
func (r *DriftDetectionResult) HasDrift() bool {
	return len(r.Added) > 0 || len(r.Modified) > 0 || len(r.Deleted) > 0
}

// TotalChanges returns the total number of changes detected
func (r *DriftDetectionResult) TotalChanges() int {
	return len(r.Added) + len(r.Modified) + len(r.Deleted)
}
