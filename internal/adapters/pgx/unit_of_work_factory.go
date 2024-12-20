package pgx_adapter

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/x0k/medods-authentication-service/internal/lib/unit_of_work"
)

type UnitOfWork struct {
	tx pgx.Tx
}

func NewUnitOfWork(ctx context.Context, pgxPool *pgxpool.Pool) (*UnitOfWork, error) {
	tx, err := pgxPool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	})
	if err != nil {
		return nil, err
	}
	return &UnitOfWork{tx: tx}, nil
}

func (uow *UnitOfWork) Tx() pgx.Tx {
	return uow.tx
}

func (uow *UnitOfWork) Commit(ctx context.Context) error {
	return uow.tx.Commit(ctx)
}

func (uow *UnitOfWork) Rollback(ctx context.Context) error {
	err := uow.tx.Rollback(ctx)
	if errors.Is(err, pgx.ErrTxClosed) {
		return nil
	}
	return err
}

func NewUnitOfWorkFactory(pgxPool *pgxpool.Pool) unit_of_work.Factory[pgx.Tx] {
	return func(ctx context.Context) (unit_of_work.UnitOfWork[pgx.Tx], error) {
		return NewUnitOfWork(ctx, pgxPool)
	}
}
