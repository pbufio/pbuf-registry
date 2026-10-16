package background

import (
	"errors"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/mocks"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProtoParsingDaemon_Run_PoisonPillDoesNotBlockValidTagsOrDriftDetection(t *testing.T) {
	metaRepo := mocks.NewMetadataRepository(t)
	driftRepo := mocks.NewDriftRepository(t)

	daemon := NewProtoParsingDaemon(metaRepo, driftRepo, log.DefaultLogger)
	assert.Equal(t, "proto parsing", daemon.Name())

	// 4 unprocessed tags:
	// - tag-1: valid v1.0.0
	// - tag-bad-syntax: invalid proto syntax (poison pill)
	// - tag-save-err: valid proto syntax, but SaveParsedProtoFiles fails
	// - tag-2: valid v2.0.0 with drift against tag-1
	metaRepo.On("GetUnprocessedTagIds", mock.Anything).
		Return([]string{"tag-1", "tag-bad-syntax", "tag-save-err", "tag-2"}, nil).
		Once()

	validProtoV1 := `syntax = "proto3"; package test.v1; message User { string id = 1; string email = 2; }`
	validProtoV2 := `syntax = "proto3"; package test.v1; message User { string id = 1; }`

	// tag-1 succeeds
	metaRepo.On("GetProtoFilesForTagId", mock.Anything, "tag-1").
		Return([]*v1.ProtoFile{{Filename: "user.proto", Content: validProtoV1}}, nil).
		Once()
	metaRepo.On("SaveParsedProtoFiles", mock.Anything, "tag-1", mock.Anything).
		Return(nil).
		Once()

	// tag-bad-syntax fails ParseProtoFilesContents
	metaRepo.On("GetProtoFilesForTagId", mock.Anything, "tag-bad-syntax").
		Return([]*v1.ProtoFile{{Filename: "broken.proto", Content: "not valid proto syntax {{{{"}}, nil).
		Once()

	// tag-save-err fails SaveParsedProtoFiles
	metaRepo.On("GetProtoFilesForTagId", mock.Anything, "tag-save-err").
		Return([]*v1.ProtoFile{{Filename: "user.proto", Content: validProtoV1}}, nil).
		Once()
	metaRepo.On("SaveParsedProtoFiles", mock.Anything, "tag-save-err", mock.Anything).
		Return(errors.New("db write error")).
		Once()

	// tag-2 succeeds despite previous failures
	metaRepo.On("GetProtoFilesForTagId", mock.Anything, "tag-2").
		Return([]*v1.ProtoFile{{Filename: "user.proto", Content: validProtoV2}}, nil).
		Once()
	metaRepo.On("SaveParsedProtoFiles", mock.Anything, "tag-2", mock.Anything).
		Return(nil).
		Once()

	// Phase 1 of drift detection: compute hashes for tags without hashes
	driftRepo.On("GetTagsWithoutHashes", mock.Anything).
		Return([]string{"tag-1", "tag-2"}, nil).
		Once()
	driftRepo.On("ComputeAndStoreHashes", mock.Anything, "tag-1").
		Return(nil).
		Once()
	driftRepo.On("ComputeAndStoreHashes", mock.Anything, "tag-2").
		Return(nil).
		Once()

	// Phase 2 of drift detection: runs for tag-1 (no previous tag) and tag-2 (drift against tag-1)
	driftRepo.On("GetTagInfo", mock.Anything, "tag-1").
		Return("mod-1", "v1.0.0", nil).
		Once()
	driftRepo.On("GetPreviousTagID", mock.Anything, "mod-1", "tag-1").
		Return("", nil).
		Once()

	driftRepo.On("GetTagInfo", mock.Anything, "tag-2").
		Return("mod-1", "v2.0.0", nil).
		Once()
	driftRepo.On("GetPreviousTagID", mock.Anything, "mod-1", "tag-2").
		Return("tag-1", nil).
		Once()
	driftRepo.On("GetFileHashesForTag", mock.Anything, "tag-2").
		Return(map[string]string{
			"user.proto":  "hash-v2",
			"added.proto": "hash-added",
		}, nil).
		Once()
	driftRepo.On("GetFileHashesForTag", mock.Anything, "tag-1").
		Return(map[string]string{
			"user.proto":    "hash-v1",
			"deleted.proto": "hash-deleted",
		}, nil).
		Once()

	metaRepo.On("GetParsedProtoFiles", mock.Anything, "tag-2").
		Return([]*model.ParsedProtoFile{
			{Filename: "user.proto", Proto: parseProtoString(t, validProtoV2)},
		}, nil).
		Once()
	metaRepo.On("GetParsedProtoFiles", mock.Anything, "tag-1").
		Return([]*model.ParsedProtoFile{
			{Filename: "user.proto", Proto: parseProtoString(t, validProtoV1)},
		}, nil).
		Once()

	driftRepo.On("SaveDriftEvents", mock.Anything, mock.MatchedBy(func(events []model.DriftEvent) bool {
		if len(events) != 3 {
			return false
		}
		byFile := make(map[string]model.DriftEvent, len(events))
		for _, ev := range events {
			byFile[ev.Filename] = ev
		}
		return byFile["added.proto"].EventType == model.DriftEventTypeAdded &&
			byFile["added.proto"].Severity == model.DriftSeverityInfo &&
			byFile["user.proto"].EventType == model.DriftEventTypeModified &&
			byFile["user.proto"].Severity == model.DriftSeverityCritical &&
			byFile["deleted.proto"].EventType == model.DriftEventTypeDeleted &&
			byFile["deleted.proto"].Severity == model.DriftSeverityCritical
	})).
		Return(nil).
		Once()

	err := daemon.Run()
	require.NoError(t, err)
}

func TestProtoParsingDaemon_Run_GetUnprocessedTagIdsError(t *testing.T) {
	metaRepo := mocks.NewMetadataRepository(t)
	driftRepo := mocks.NewDriftRepository(t)

	daemon := NewProtoParsingDaemon(metaRepo, driftRepo, log.DefaultLogger)

	metaRepo.On("GetUnprocessedTagIds", mock.Anything).
		Return(nil, errors.New("db connection error")).
		Once()

	err := daemon.Run()
	require.EqualError(t, err, "db connection error")
}
