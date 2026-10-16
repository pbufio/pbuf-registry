package server

import (
	"context"
	"log"
	"net"
	"testing"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/mocks"
	"github.com/stretchr/testify/mock"
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

	// Negative pageSize should clamp to defaultPageSize (10)
	registryRepository.On("ListModules", mock.Anything, 10, "tok-1").
		Return([]*v1.Module{{Name: "m1"}}, "next-1", nil).Once()

	resp, err := srv.ListModules(ctx, &v1.ListModulesRequest{PageSize: -10, PageToken: "tok-1"})
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

