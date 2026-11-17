package server

import (
	"context"
	"log"
	"net"
	"testing"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func setup(ctx context.Context) (v1.RegistryClient, func()) {
	buffer := 101024 * 1024
	lis := bufconn.Listen(buffer)

	baseServer := grpc.NewServer()

	registryRepository := &mocks.RegistryRepository{}
	registryRepository.On("RegisterModule", mock.Anything, "hello").Return(nil)

	metadataRepository := &mocks.MetadataRepository{}

	v1.RegisterRegistryServer(baseServer, NewRegistryServer(registryRepository, metadataRepository, nil))
	go func() {
		if err := baseServer.Serve(lis); err != nil {
			log.Printf("error serving server: %v", err)
		}
	}()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("error connecting to server: %v", err)
	}

	closer := func() {
		err := lis.Close()
		if err != nil {
			log.Printf("error closing listener: %v", err)
		}
		baseServer.Stop()
	}

	client := v1.NewRegistryClient(conn)

	return client, closer
}

func TestRegisterService(t *testing.T) {
	ctx := context.Background()
	client, closer := setup(ctx)
	defer closer()

	module, err := client.RegisterModule(ctx, &v1.RegisterModuleRequest{Name: "hello"})
	if err != nil {
		t.Errorf("error registering module: %v", err)
	}

	if module.Name != "hello" {
		t.Errorf("expected module name to be hello, got %s", module.Name)
	}
}

func TestListModules_PageSizeClamping(t *testing.T) {
	ctx := context.Background()

	registryRepository := &mocks.RegistryRepository{}
	metadataRepository := &mocks.MetadataRepository{}
	srv := NewRegistryServer(registryRepository, metadataRepository, nil)

	// Zero or negative pageSize should clamp to defaultPageSize (50) and preserve populated metadata
	populated := &v1.Module{
		Id:        "id-1",
		Name:      "m1",
		Tags:      []string{"v1.1.0", "v1.0.0"},
		DraftTags: []string{"v1.2.0-rc1"},
		Packages:  []string{"acme.m1.v1"},
	}
	registryRepository.On("ListModules", mock.Anything, 50, "").
		Return([]*v1.Module{populated}, "", nil).Once()

	resp, err := srv.ListModules(ctx, &v1.ListModulesRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Modules) != 1 || len(resp.Modules[0].Tags) != 2 || len(resp.Modules[0].DraftTags) != 1 || len(resp.Modules[0].Packages) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}

	registryRepository.On("ListModules", mock.Anything, 50, "tok-1").
		Return([]*v1.Module{{Name: "m1"}}, "next-1", nil).Once()

	resp, err = srv.ListModules(ctx, &v1.ListModulesRequest{PageSize: -10, PageToken: "tok-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Modules) != 1 || resp.NextPageToken != "next-1" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	// Excessive pageSize (>1000) should clamp to 1000
	registryRepository.On("ListModules", mock.Anything, 1000, "tok-2").
		Return([]*v1.Module{{Name: "m2"}}, "", nil).Once()

	resp, err = srv.ListModules(ctx, &v1.ListModulesRequest{PageSize: 99999, PageToken: "tok-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Modules) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}

	registryRepository.AssertExpectations(t)
}

func TestPushModule_AtomicWithDependencies(t *testing.T) {
	ctx := context.Background()

	registryRepository := &mocks.RegistryRepository{}
	metadataRepository := &mocks.MetadataRepository{}
	srv := NewRegistryServer(registryRepository, metadataRepository, nil)

	protoFiles := []*v1.ProtoFile{
		{Filename: "hello.proto", Content: `syntax = "proto3"; package hello; message Hello {}`},
	}
	deps := []*v1.Dependency{
		{Name: "dep/mod", Tag: "v1.0.0"},
	}

	registryRepository.On("PushModule", mock.Anything, "org/svc", "v1.0.0", protoFiles, deps[0]).
		Return(&v1.Module{Name: "org/svc", Tags: []string{"v1.0.0"}}, nil).Once()

	mod, err := srv.PushModule(ctx, &v1.PushModuleRequest{
		ModuleName:   "org/svc",
		Tag:          "v1.0.0",
		Protofiles:   protoFiles,
		Dependencies: deps,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod.Name != "org/svc" {
		t.Fatalf("expected module name org/svc, got %s", mod.Name)
	}

	registryRepository.AssertExpectations(t)
}

func TestRegistryServer_ErrorCodesAndDraftFallback(t *testing.T) {
	ctx := context.Background()
	registryRepository := &mocks.RegistryRepository{}
	metadataRepository := &mocks.MetadataRepository{}
	srv := NewRegistryServer(registryRepository, metadataRepository, nil)

	registryRepository.On("GetModuleWithDrafts", mock.Anything, "missing/mod", false).Return(nil, nil).Once()
	_, err := srv.GetModule(ctx, &v1.GetModuleRequest{Name: "missing/mod"})
	require.Error(t, err)
	assert.True(t, kerrors.IsNotFound(err))

	registryRepository.On("GetModuleWithDrafts", mock.Anything, "org/with-drafts", false).
		Return(&v1.Module{Name: "org/with-drafts"}, nil).Once()
	noDraftsMod, err := srv.GetModule(ctx, &v1.GetModuleRequest{Name: "org/with-drafts", IncludeDraftTags: false})
	require.NoError(t, err)
	assert.Empty(t, noDraftsMod.DraftTags)

	registryRepository.On("GetModuleWithDrafts", mock.Anything, "org/with-drafts", true).
		Return(&v1.Module{Name: "org/with-drafts", DraftTags: []string{"v2.0.0-rc1"}}, nil).Once()
	withDraftsMod, err := srv.GetModule(ctx, &v1.GetModuleRequest{Name: "org/with-drafts", IncludeDraftTags: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"v2.0.0-rc1"}, withDraftsMod.DraftTags)

	registryRepository.On("ListModules", mock.Anything, 50, "!!!bad").
		Return(nil, "", data.ErrInvalidPageToken).Once()
	_, err = srv.ListModules(ctx, &v1.ListModulesRequest{PageToken: "!!!bad"})
	require.Error(t, err)
	assert.True(t, kerrors.IsBadRequest(err))

	_, err = srv.PushModule(ctx, &v1.PushModuleRequest{
		ModuleName: "org/svc",
		Tag:        "v1.0.0",
		Protofiles: []*v1.ProtoFile{{Filename: "../escape.proto", Content: `syntax = "proto3";`}},
	})
	require.Error(t, err)
	assert.True(t, kerrors.IsBadRequest(err))

	registryRepository.On("PullModule", mock.Anything, "org/svc", "v1.0.0-rc1").
		Return(nil, nil, data.ErrTagNotFound).Once()
	registryRepository.On("PullDraftModule", mock.Anything, "org/svc", "v1.0.0-rc1").
		Return(nil, nil, data.ErrTagNotFound).Once()
	_, err = srv.PullModule(ctx, &v1.PullModuleRequest{Name: "org/svc", Tag: "v1.0.0-rc1"})
	require.Error(t, err)
	assert.True(t, kerrors.IsNotFound(err))

	registryRepository.On("DeleteModule", mock.Anything, "missing/mod").
		Return(data.ErrModuleNotFound).Once()
	_, err = srv.DeleteModule(ctx, &v1.DeleteModuleRequest{Name: "missing/mod"})
	require.Error(t, err)
	assert.True(t, kerrors.IsNotFound(err))

	registryRepository.On("DeleteModuleTag", mock.Anything, "org/svc", "v9.9.9").
		Return(data.ErrTagNotFound).Once()
	_, err = srv.DeleteModuleTag(ctx, &v1.DeleteModuleTagRequest{Name: "org/svc", Tag: "v9.9.9"})
	require.Error(t, err)
	assert.True(t, kerrors.IsNotFound(err))

	registryRepository.AssertExpectations(t)
}

