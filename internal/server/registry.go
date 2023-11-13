package server

import (
	"context"
	"errors"

	"github.com/google/martian/log"
	v1 "github.com/pbufio/pbuf-registry/gen/v1"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/utils"
)

const (
	defaultPageSize = 10
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

func (r *RegistryServer) RegisterModule(ctx context.Context, request *v1.RegisterModuleRequest) (*v1.Module, error) {
	name := request.Name

	if name == "" {
		return nil, errors.New("name cannot be empty")
	}

	err := r.registryRepository.RegisterModule(ctx, name)
	if err != nil {
		log.Infof("error registering module: %v", err)
		return nil, err
	}

	return &v1.Module{
		Name: name,
	}, nil
}

func (r *RegistryServer) GetModule(ctx context.Context, request *v1.GetModuleRequest) (*v1.Module, error) {
	name := request.Name

	if name == "" {
		return nil, errors.New("name cannot be empty")
	}

	module, err := r.registryRepository.GetModule(ctx, name)
	if err != nil {
		log.Infof("error getting module: %v", err)
		return nil, err
	}

	if module == nil {
		return nil, errors.New("module not found")
	}

	return module, nil
}

func (r *RegistryServer) ListModules(ctx context.Context, request *v1.ListModulesRequest) (*v1.ListModulesResponse, error) {
	pageSize := int(request.PageSize)
	if pageSize == 0 {
		pageSize = defaultPageSize
	}

	modules, nextPageToken, err := r.registryRepository.ListModules(ctx, pageSize, request.PageToken)
	if err != nil {
		log.Infof("error listing modules: %v", err)
		return nil, err
	}

	return &v1.ListModulesResponse{
		Modules:       modules,
		NextPageToken: nextPageToken,
	}, nil
}

func (r *RegistryServer) DeleteModule(ctx context.Context, request *v1.DeleteModuleRequest) (*v1.DeleteModuleResponse, error) {
	name := request.Name

	if name == "" {
		return nil, errors.New("name cannot be empty")
	}

	err := r.registryRepository.DeleteModule(ctx, name)
	if err != nil {
		log.Infof("error deleting module: %v", err)
		return nil, err
	}

	return &v1.DeleteModuleResponse{
		Name: name,
	}, nil
}

func (r *RegistryServer) PushModule(ctx context.Context, request *v1.PushModuleRequest) (*v1.Module, error) {
	name := request.ModuleName
	if name == "" {
		return nil, errors.New("name cannot be empty")
	}

	tag := request.Tag
	if tag == "" {
		return nil, errors.New("tag cannot be empty")
	}

	err := utils.ValidateProtoFiles(request.Protofiles)
	if err != nil {
		return nil, err
	}

	module, err := r.registryRepository.PushModule(ctx, name, tag, request.Protofiles)
	if err != nil {
		log.Infof("error pushing module: %v", err)
		return nil, err
	}

	err = r.registryRepository.AddModuleDependencies(ctx, name, tag, request.Dependencies)
	if err != nil {
		log.Infof("error while adding dependencies for module: %v", err)
		return nil, err
	}

	return module, nil
}

func (r *RegistryServer) PullModule(ctx context.Context, request *v1.PullModuleRequest) (*v1.PullModuleResponse, error) {
	name := request.Name
	if name == "" {
		return nil, errors.New("name cannot be empty")
	}

	tag := request.Tag
	if tag == "" {
		return nil, errors.New("tag cannot be empty")
	}

	module, protoFiles, err := r.registryRepository.PullModule(ctx, name, tag)
	if err != nil {
		log.Infof("error pulling module: %v", err)
		return nil, err
	}

	return &v1.PullModuleResponse{
		Module:     module,
		Protofiles: protoFiles,
	}, nil
}

func (r *RegistryServer) DeleteModuleTag(ctx context.Context, request *v1.DeleteModuleTagRequest) (*v1.DeleteModuleTagResponse, error) {
	name := request.Name
	if name == "" {
		return nil, errors.New("name cannot be empty")
	}

	tag := request.Tag
	if tag == "" {
		return nil, errors.New("tag cannot be empty")
	}

	err := r.registryRepository.DeleteModuleTag(ctx, name, tag)
	if err != nil {
		log.Infof("error deleting module tag: %v", err)
		return nil, err
	}

	return &v1.DeleteModuleTagResponse{
		Name: name,
		Tag:  tag,
	}, nil
}

func (r *RegistryServer) GetModuleDependencies(ctx context.Context, request *v1.GetModuleDependenciesRequest) (*v1.GetModuleDependenciesResponse, error) {
	name := request.Name
	if name == "" {
		return nil, errors.New("name cannot be empty")
	}

	dependencies, err := r.registryRepository.GetModuleDependencies(ctx, name, request.Tag)
	if err != nil {
		log.Infof("error getting module dependencies: %v", err)
		return nil, err
	}

	return &v1.GetModuleDependenciesResponse{
		Dependencies: dependencies,
	}, nil
}
