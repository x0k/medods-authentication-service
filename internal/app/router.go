package app

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	http_adapters "github.com/x0k/medods-authentication-service/internal/adapters/http"
	"github.com/x0k/medods-authentication-service/internal/auth"
	"github.com/x0k/medods-authentication-service/internal/lib/logger"
)

func NewRouter(
	log *logger.Logger,
	pgxPool *pgxpool.Pool,
	authSecret []byte,
	usersRepo auth.UsersRepository,
	messagesSender auth.MessagesSender,
) http.Handler {
	router := http.NewServeMux()
	router.Handle("/auth/",
		http.StripPrefix("/auth", auth.New(
			log.With(slog.String("module", "auth")),
			pgxPool,
			authSecret,
			usersRepo,
			messagesSender,
		),
		))
	sLog := log.With(slog.String("component", "http_server"))
	return http_adapters.Recover(
		sLog,
		http_adapters.Logging(
			sLog,
			router,
		),
	)
}
