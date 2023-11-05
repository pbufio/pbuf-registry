package server

import (
	"context"

	"github.com/google/martian/log"
	v1 "github.com/pbufio/pbuf-registry/gen/v1"
	"github.com/pbufio/pbuf-registry/internal/data"
)

type RegistryServer struct {
	v1.UnimplementedRegistryServer
	registryRepository data.RegistryRepository
}

func NewRegistryServer(registryRepository data.RegistryRepository) *RegistryServer {
	return &RegistryServer{
		registryRepository: registryRepository,
	}
}

func (r *RegistryServer) ListModules(ctx context.Context, request *v1.ListModulesRequest) (*v1.ListModulesResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RegistryServer) GetModule(ctx context.Context, request *v1.GetModuleRequest) (*v1.Module, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RegistryServer) RegisterModule(ctx context.Context, request *v1.RegisterModuleRequest) (*v1.Module, error) {
	err := r.registryRepository.RegisterModule(ctx, request.Name)
	if err != nil {
		log.Infof("error registering module: %v", err)
		return nil, err
	}

	return &v1.Module{
		Name: request.Name,
	}, nil
}

func (r *RegistryServer) PullModule(ctx context.Context, request *v1.PullModuleRequest) (*v1.Module, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RegistryServer) PushModule(ctx context.Context, request *v1.PushModuleRequest) (*v1.Module, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RegistryServer) DeleteModule(ctx context.Context, request *v1.DeleteModuleRequest) (*v1.DeleteModuleResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RegistryServer) DeleteModuleTag(ctx context.Context, request *v1.DeleteModuleTagRequest) (*v1.DeleteModuleTagResponse, error) {
	//TODO implement me
	panic("implement me")
}
