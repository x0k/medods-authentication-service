package auth

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/x0k/medods-authentication-service/internal/lib/logger"
	"github.com/x0k/medods-authentication-service/internal/lib/unit_of_work"
	"github.com/x0k/medods-authentication-service/internal/shared"
)

type refreshTokensRepository struct {
	log  *logger.Logger
	pool *pgxpool.Pool
}

func newRefreshTokensRepository(log *logger.Logger, pool *pgxpool.Pool) *refreshTokensRepository {
	return &refreshTokensRepository{
		log:  log,
		pool: pool,
	}
}

const saveTokeHashQuery = `INSERT INTO refresh_token (user_id, device_id, token_hash)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, device_id) DO UPDATE SET token_hash = $3`

func (r *refreshTokensRepository) UpsertTokenHash(
	ctx context.Context,
	userId uuid.UUID,
	deviceId DeviceId,
	tokenHash []byte,
) error {
	args := []any{userId, deviceId[:], tokenHash}
	r.log.Debug(ctx, "executing query", slog.String("query", saveTokeHashQuery), slog.Any("args", args))
	_, err := r.pool.Exec(ctx, saveTokeHashQuery, args...)
	return err
}

const tokenHashesQuery = `SELECT token_hash FROM refresh_token WHERE user_id = $1 AND device_id = $2`

func (r *refreshTokensRepository) TokenHash(
	ctx context.Context,
	uow unit_of_work.UnitOfWork[pgx.Tx],
	userId uuid.UUID,
	deviceId DeviceId,
) ([]byte, error) {
	args := []any{userId, deviceId[:]}
	r.log.Debug(ctx, "executing query", slog.String("query", tokenHashesQuery), slog.Any("args", args))
	row := uow.Tx().QueryRow(ctx, tokenHashesQuery, args...)
	var tokenHash []byte
	err := row.Scan(&tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, shared.ErrNotFound
		}
		return nil, err
	}
	return tokenHash, nil
}

const replaceTokenHashQuery = `UPDATE refresh_token SET device_id = $3, token_hash = $4
WHERE user_id = $1 AND device_id = $2`

func (r *refreshTokensRepository) UpdateTokenHash(
	ctx context.Context,
	uow unit_of_work.UnitOfWork[pgx.Tx],
	userId uuid.UUID,
	oldDeviceId DeviceId,
	newDeviceId DeviceId,
	tokenHash []byte,
) error {
	args := []any{userId, oldDeviceId[:], newDeviceId[:], tokenHash}
	r.log.Debug(ctx, "executing query", slog.String("query", replaceTokenHashQuery), slog.Any("args", args))
	cmd, err := uow.Tx().Exec(ctx, replaceTokenHashQuery, args...)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return shared.ErrNotFound
	}
	return nil
}
