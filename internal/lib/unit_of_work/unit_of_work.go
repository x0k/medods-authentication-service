package unit_of_work

import "context"

type UnitOfWork[T any] interface {
	Tx() T
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type Factory[T any] func(ctx context.Context) (UnitOfWork[T], error)
