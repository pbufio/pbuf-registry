package server

import (
	"context"
	"errors"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/mocks"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/pbufio/pbuf-registry/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMetadataServer_GetMetadata_Validation(t *testing.T) {
	regRepo := mocks.NewRegistryRepository(t)
	metaRepo := mocks.NewMetadataRepository(t)
	srv := NewMetadataServer(regRepo, metaRepo, log.DefaultLogger)
	ctx := context.Background()

	t.Run("missing module name", func(t *testing.T) {
		resp, err := srv.GetMetadata(ctx, &v1.GetMetadataRequest{Name: "", Tag: "v1.0.0"})
		assert.Nil(t, resp)
		require.EqualError(t, err, "module name is required")
	})

	t.Run("missing tag", func(t *testing.T) {
		resp, err := srv.GetMetadata(ctx, &v1.GetMetadataRequest{Name: "acme/orders", Tag: ""})
		assert.Nil(t, resp)
		require.EqualError(t, err, "tag is required")
	})
}

func TestMetadataServer_GetMetadata_NotFoundAndErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("GetModuleTagId error", func(t *testing.T) {
		regRepo := mocks.NewRegistryRepository(t)
		metaRepo := mocks.NewMetadataRepository(t)
		srv := NewMetadataServer(regRepo, metaRepo, log.DefaultLogger)

		regRepo.On("GetModuleTagId", mock.Anything, "acme/orders", "v1.0.0").
			Return("", errors.New("db error")).Once()

		resp, err := srv.GetMetadata(ctx, &v1.GetMetadataRequest{Name: "acme/orders", Tag: "v1.0.0"})
		assert.Nil(t, resp)
		require.EqualError(t, err, "db error")
	})

	t.Run("module tag not found", func(t *testing.T) {
		regRepo := mocks.NewRegistryRepository(t)
		metaRepo := mocks.NewMetadataRepository(t)
		srv := NewMetadataServer(regRepo, metaRepo, log.DefaultLogger)

		regRepo.On("GetModuleTagId", mock.Anything, "acme/orders", "v1.0.0").
			Return("", nil).Once()

		resp, err := srv.GetMetadata(ctx, &v1.GetMetadataRequest{Name: "acme/orders", Tag: "v1.0.0"})
		assert.Nil(t, resp)
		require.EqualError(t, err, "module tag not found")
	})

	t.Run("GetTagMetaByTagId error", func(t *testing.T) {
		regRepo := mocks.NewRegistryRepository(t)
		metaRepo := mocks.NewMetadataRepository(t)
		srv := NewMetadataServer(regRepo, metaRepo, log.DefaultLogger)

		regRepo.On("GetModuleTagId", mock.Anything, "acme/orders", "v1.0.0").
			Return("tag-123", nil).Once()
		metaRepo.On("GetTagMetaByTagId", mock.Anything, "tag-123").
			Return((*model.TagMeta)(nil), errors.New("meta db error")).Once()

		resp, err := srv.GetMetadata(ctx, &v1.GetMetadataRequest{Name: "acme/orders", Tag: "v1.0.0"})
		assert.Nil(t, resp)
		require.EqualError(t, err, "meta db error")
	})

	t.Run("tag meta not found", func(t *testing.T) {
		regRepo := mocks.NewRegistryRepository(t)
		metaRepo := mocks.NewMetadataRepository(t)
		srv := NewMetadataServer(regRepo, metaRepo, log.DefaultLogger)

		regRepo.On("GetModuleTagId", mock.Anything, "acme/orders", "v1.0.0").
			Return("tag-123", nil).Once()
		metaRepo.On("GetTagMetaByTagId", mock.Anything, "tag-123").
			Return((*model.TagMeta)(nil), nil).Once()

		resp, err := srv.GetMetadata(ctx, &v1.GetMetadataRequest{Name: "acme/orders", Tag: "v1.0.0"})
		assert.Nil(t, resp)
		require.EqualError(t, err, "tag meta not found")
	})

	t.Run("GetParsedProtoFiles error", func(t *testing.T) {
		regRepo := mocks.NewRegistryRepository(t)
		metaRepo := mocks.NewMetadataRepository(t)
		srv := NewMetadataServer(regRepo, metaRepo, log.DefaultLogger)

		regRepo.On("GetModuleTagId", mock.Anything, "acme/orders", "v1.0.0").
			Return("tag-123", nil).Once()
		metaRepo.On("GetTagMetaByTagId", mock.Anything, "tag-123").
			Return(&model.TagMeta{Packages: []string{"acme.orders.v1"}}, nil).Once()
		metaRepo.On("GetParsedProtoFiles", mock.Anything, "tag-123").
			Return(([]*model.ParsedProtoFile)(nil), errors.New("parsed files db error")).Once()

		resp, err := srv.GetMetadata(ctx, &v1.GetMetadataRequest{Name: "acme/orders", Tag: "v1.0.0"})
		assert.Nil(t, resp)
		require.EqualError(t, err, "parsed files db error")
	})
}

func TestMetadataServer_GetMetadata_Success(t *testing.T) {
	ctx := context.Background()
	regRepo := mocks.NewRegistryRepository(t)
	metaRepo := mocks.NewMetadataRepository(t)
	srv := NewMetadataServer(regRepo, metaRepo, log.DefaultLogger)

	parsedFiles, err := utils.ParseProtoFilesContents([]*v1.ProtoFile{
		{
			Filename: "orders.proto",
			Content: `
				syntax = "proto3";
				package acme.orders.v1;

				message OrderRequest {
					string order_id = 1;
				}
				message OrderResponse {
					string status = 1;
				}
				service OrderService {
					rpc GetOrder(OrderRequest) returns (OrderResponse);
				}
			`,
		},
	})
	require.NoError(t, err)

	regRepo.On("GetModuleTagId", mock.Anything, "acme/orders", "v1.0.0").
		Return("tag-123", nil).Once()
	metaRepo.On("GetTagMetaByTagId", mock.Anything, "tag-123").
		Return(&model.TagMeta{Packages: []string{"acme.orders.v1"}}, nil).Once()
	metaRepo.On("GetParsedProtoFiles", mock.Anything, "tag-123").
		Return(parsedFiles, nil).Once()

	resp, err := srv.GetMetadata(ctx, &v1.GetMetadataRequest{Name: "acme/orders", Tag: "v1.0.0"})
	require.NoError(t, err)
	require.Len(t, resp.Packages, 1)
	assert.Equal(t, "acme.orders.v1", resp.Packages[0].Name)
	require.Len(t, resp.Packages[0].ProtoFiles, 1)
	assert.Equal(t, "orders.proto", resp.Packages[0].ProtoFiles[0].Filename)
	assert.Len(t, resp.Packages[0].ProtoFiles[0].Messages, 2)
	assert.Len(t, resp.Packages[0].ProtoFiles[0].Services, 1)
}
