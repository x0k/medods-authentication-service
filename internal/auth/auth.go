package auth

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	pgx_adapter "github.com/x0k/medods-authentication-service/internal/adapters/pgx"
	"github.com/x0k/medods-authentication-service/internal/lib/logger"
)

func New(
	log *logger.Logger,
	pgxPool *pgxpool.Pool,
	secret []byte,
	usersRepo UsersRepository,
	sender MessagesSender,
) *http.ServeMux {
	refreshTokensRepository := newRefreshTokensRepository(
		log.With(slog.String("component", "refresh_tokens_repository")),
		pgxPool,
	)
	service := newService(
		log.With(slog.String("component", "service")),
		secret,
		usersRepo,
		refreshTokensRepository,
		sender,
		pgx_adapter.NewUnitOfWorkFactory(pgxPool),
	)
	controller := newController(
		log.With(slog.String("component", "controller")),
		service,
	)
	return newRouter(controller)
}
