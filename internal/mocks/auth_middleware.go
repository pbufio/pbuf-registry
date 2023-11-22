// Code generated by mockery v2.37.1. DO NOT EDIT.

package mocks

import (
	middleware "github.com/go-kratos/kratos/v2/middleware"
	mock "github.com/stretchr/testify/mock"
)

// AuthMiddleware is an autogenerated mock type for the AuthMiddleware type
type AuthMiddleware struct {
	mock.Mock
}

// NewAuthMiddleware provides a mock function with given fields:
func (_m *AuthMiddleware) NewAuthMiddleware() middleware.Middleware {
	ret := _m.Called()

	var r0 middleware.Middleware
	if rf, ok := ret.Get(0).(func() middleware.Middleware); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(middleware.Middleware)
		}
	}

	return r0
}

// NewAuthMiddleware creates a new instance of AuthMiddleware. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewAuthMiddleware(t interface {
	mock.TestingT
	Cleanup(func())
}) *AuthMiddleware {
	mock := &AuthMiddleware{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}