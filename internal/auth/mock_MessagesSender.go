// Code generated by mockery v2.46.3. DO NOT EDIT.

package auth

import (
	context "context"

	uuid "github.com/google/uuid"
	mock "github.com/stretchr/testify/mock"
)

// MockMessagesSender is an autogenerated mock type for the MessagesSender type
type MockMessagesSender struct {
	mock.Mock
}

type MockMessagesSender_Expecter struct {
	mock *mock.Mock
}

func (_m *MockMessagesSender) EXPECT() *MockMessagesSender_Expecter {
	return &MockMessagesSender_Expecter{mock: &_m.Mock}
}

// SendWarning provides a mock function with given fields: ctx, userId, message
func (_m *MockMessagesSender) SendWarning(ctx context.Context, userId uuid.UUID, message string) error {
	ret := _m.Called(ctx, userId, message)

	if len(ret) == 0 {
		panic("no return value specified for SendWarning")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string) error); ok {
		r0 = rf(ctx, userId, message)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMessagesSender_SendWarning_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SendWarning'
type MockMessagesSender_SendWarning_Call struct {
	*mock.Call
}

// SendWarning is a helper method to define mock.On call
//   - ctx context.Context
//   - userId uuid.UUID
//   - message string
func (_e *MockMessagesSender_Expecter) SendWarning(ctx interface{}, userId interface{}, message interface{}) *MockMessagesSender_SendWarning_Call {
	return &MockMessagesSender_SendWarning_Call{Call: _e.mock.On("SendWarning", ctx, userId, message)}
}

func (_c *MockMessagesSender_SendWarning_Call) Run(run func(ctx context.Context, userId uuid.UUID, message string)) *MockMessagesSender_SendWarning_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(uuid.UUID), args[2].(string))
	})
	return _c
}

func (_c *MockMessagesSender_SendWarning_Call) Return(_a0 error) *MockMessagesSender_SendWarning_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessagesSender_SendWarning_Call) RunAndReturn(run func(context.Context, uuid.UUID, string) error) *MockMessagesSender_SendWarning_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockMessagesSender creates a new instance of MockMessagesSender. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockMessagesSender(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockMessagesSender {
	mock := &MockMessagesSender{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
