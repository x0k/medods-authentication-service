package app

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/x0k/medods-authentication-service/internal/auth"
	"github.com/x0k/medods-authentication-service/internal/lib/logger"
)

func newRouter(
	log *logger.Logger,
	pgxPool *pgxpool.Pool,
	authSecret []byte,
	usersRepo auth.UsersRepository,
	messagesSender auth.MessagesSender,
) *http.ServeMux {
	router := http.NewServeMux()
	router.Handle("/auth", auth.New(
		log.With(slog.String("module", "auth")),
		pgxPool,
		authSecret,
		usersRepo,
		messagesSender,
	))
	return router
}
