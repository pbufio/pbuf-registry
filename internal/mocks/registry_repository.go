// Code generated by mockery v2.38.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
)

// RegistryRepository is an autogenerated mock type for the RegistryRepository type
type RegistryRepository struct {
	mock.Mock
}

// AddModuleDependencies provides a mock function with given fields: ctx, name, tag, dependencies
func (_m *RegistryRepository) AddModuleDependencies(ctx context.Context, name string, tag string, dependencies []*v1.Dependency) error {
	ret := _m.Called(ctx, name, tag, dependencies)

	if len(ret) == 0 {
		panic("no return value specified for AddModuleDependencies")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, []*v1.Dependency) error); ok {
		r0 = rf(ctx, name, tag, dependencies)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteModule provides a mock function with given fields: ctx, name
func (_m *RegistryRepository) DeleteModule(ctx context.Context, name string) error {
	ret := _m.Called(ctx, name)

	if len(ret) == 0 {
		panic("no return value specified for DeleteModule")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, name)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteModuleTag provides a mock function with given fields: ctx, name, tag
func (_m *RegistryRepository) DeleteModuleTag(ctx context.Context, name string, tag string) error {
	ret := _m.Called(ctx, name, tag)

	if len(ret) == 0 {
		panic("no return value specified for DeleteModuleTag")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) error); ok {
		r0 = rf(ctx, name, tag)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteObsoleteDraftTags provides a mock function with given fields: ctx
func (_m *RegistryRepository) DeleteObsoleteDraftTags(ctx context.Context) error {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for DeleteObsoleteDraftTags")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetModule provides a mock function with given fields: ctx, name
func (_m *RegistryRepository) GetModule(ctx context.Context, name string) (*v1.Module, error) {
	ret := _m.Called(ctx, name)

	if len(ret) == 0 {
		panic("no return value specified for GetModule")
	}

	var r0 *v1.Module
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*v1.Module, error)); ok {
		return rf(ctx, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *v1.Module); ok {
		r0 = rf(ctx, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.Module)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetModuleDependencies provides a mock function with given fields: ctx, name, tag
func (_m *RegistryRepository) GetModuleDependencies(ctx context.Context, name string, tag string) ([]*v1.Dependency, error) {
	ret := _m.Called(ctx, name, tag)

	if len(ret) == 0 {
		panic("no return value specified for GetModuleDependencies")
	}

	var r0 []*v1.Dependency
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) ([]*v1.Dependency, error)); ok {
		return rf(ctx, name, tag)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) []*v1.Dependency); ok {
		r0 = rf(ctx, name, tag)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1.Dependency)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, name, tag)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetModuleTagId provides a mock function with given fields: ctx, moduleName, tag
func (_m *RegistryRepository) GetModuleTagId(ctx context.Context, moduleName string, tag string) (string, error) {
	ret := _m.Called(ctx, moduleName, tag)

	if len(ret) == 0 {
		panic("no return value specified for GetModuleTagId")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (string, error)); ok {
		return rf(ctx, moduleName, tag)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) string); ok {
		r0 = rf(ctx, moduleName, tag)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, moduleName, tag)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListModules provides a mock function with given fields: ctx, pageSize, token
func (_m *RegistryRepository) ListModules(ctx context.Context, pageSize int, token string) ([]*v1.Module, string, error) {
	ret := _m.Called(ctx, pageSize, token)

	if len(ret) == 0 {
		panic("no return value specified for ListModules")
	}

	var r0 []*v1.Module
	var r1 string
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, int, string) ([]*v1.Module, string, error)); ok {
		return rf(ctx, pageSize, token)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int, string) []*v1.Module); ok {
		r0 = rf(ctx, pageSize, token)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1.Module)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int, string) string); ok {
		r1 = rf(ctx, pageSize, token)
	} else {
		r1 = ret.Get(1).(string)
	}

	if rf, ok := ret.Get(2).(func(context.Context, int, string) error); ok {
		r2 = rf(ctx, pageSize, token)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// PullDraftModule provides a mock function with given fields: ctx, name, tag
func (_m *RegistryRepository) PullDraftModule(ctx context.Context, name string, tag string) (*v1.Module, []*v1.ProtoFile, error) {
	ret := _m.Called(ctx, name, tag)

	if len(ret) == 0 {
		panic("no return value specified for PullDraftModule")
	}

	var r0 *v1.Module
	var r1 []*v1.ProtoFile
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*v1.Module, []*v1.ProtoFile, error)); ok {
		return rf(ctx, name, tag)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *v1.Module); ok {
		r0 = rf(ctx, name, tag)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.Module)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) []*v1.ProtoFile); ok {
		r1 = rf(ctx, name, tag)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).([]*v1.ProtoFile)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, string, string) error); ok {
		r2 = rf(ctx, name, tag)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// PullModule provides a mock function with given fields: ctx, name, tag
func (_m *RegistryRepository) PullModule(ctx context.Context, name string, tag string) (*v1.Module, []*v1.ProtoFile, error) {
	ret := _m.Called(ctx, name, tag)

	if len(ret) == 0 {
		panic("no return value specified for PullModule")
	}

	var r0 *v1.Module
	var r1 []*v1.ProtoFile
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*v1.Module, []*v1.ProtoFile, error)); ok {
		return rf(ctx, name, tag)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *v1.Module); ok {
		r0 = rf(ctx, name, tag)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.Module)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) []*v1.ProtoFile); ok {
		r1 = rf(ctx, name, tag)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).([]*v1.ProtoFile)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, string, string) error); ok {
		r2 = rf(ctx, name, tag)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// PushDraftModule provides a mock function with given fields: ctx, name, tag, protofiles, dependencies
func (_m *RegistryRepository) PushDraftModule(ctx context.Context, name string, tag string, protofiles []*v1.ProtoFile, dependencies []*v1.Dependency) (*v1.Module, error) {
	ret := _m.Called(ctx, name, tag, protofiles, dependencies)

	if len(ret) == 0 {
		panic("no return value specified for PushDraftModule")
	}

	var r0 *v1.Module
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, []*v1.ProtoFile, []*v1.Dependency) (*v1.Module, error)); ok {
		return rf(ctx, name, tag, protofiles, dependencies)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, []*v1.ProtoFile, []*v1.Dependency) *v1.Module); ok {
		r0 = rf(ctx, name, tag, protofiles, dependencies)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.Module)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, []*v1.ProtoFile, []*v1.Dependency) error); ok {
		r1 = rf(ctx, name, tag, protofiles, dependencies)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PushModule provides a mock function with given fields: ctx, name, tag, protofiles
func (_m *RegistryRepository) PushModule(ctx context.Context, name string, tag string, protofiles []*v1.ProtoFile) (*v1.Module, error) {
	ret := _m.Called(ctx, name, tag, protofiles)

	if len(ret) == 0 {
		panic("no return value specified for PushModule")
	}

	var r0 *v1.Module
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, []*v1.ProtoFile) (*v1.Module, error)); ok {
		return rf(ctx, name, tag, protofiles)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, []*v1.ProtoFile) *v1.Module); ok {
		r0 = rf(ctx, name, tag, protofiles)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.Module)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, []*v1.ProtoFile) error); ok {
		r1 = rf(ctx, name, tag, protofiles)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RegisterModule provides a mock function with given fields: ctx, moduleName
func (_m *RegistryRepository) RegisterModule(ctx context.Context, moduleName string) error {
	ret := _m.Called(ctx, moduleName)

	if len(ret) == 0 {
		panic("no return value specified for RegisterModule")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, moduleName)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewRegistryRepository creates a new instance of RegistryRepository. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewRegistryRepository(t interface {
	mock.TestingT
	Cleanup(func())
}) *RegistryRepository {
	mock := &RegistryRepository{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
