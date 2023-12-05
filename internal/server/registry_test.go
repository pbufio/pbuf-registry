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

	conn, err := grpc.DialContext(ctx, "",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
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
