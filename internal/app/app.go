package app

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wneessen/go-mail"
	pgx_adapter "github.com/x0k/medods-authentication-service/internal/adapters/pgx"
	"github.com/x0k/medods-authentication-service/internal/lib/logger/sl"
	email_messages_sender "github.com/x0k/medods-authentication-service/internal/messages_sender/email"
	"github.com/x0k/medods-authentication-service/internal/users"
)

func Run(configPath string) {
	cfg := mustLoadConfig(configPath)
	log := mustNewLogger(&cfg.Logger)
	ctx := context.Background()

	if err := pgx_adapter.Migrate(
		ctx,
		log.Logger.With(slog.String("component", "pgx_migrate")),
		cfg.Postgres.ConnectionURI,
		cfg.Postgres.MigrationsURI,
	); err != nil {
		log.Error(ctx, "cannot migrate database", sl.Err(err))
		os.Exit(1)
	}

	pgxPool, err := pgxpool.New(ctx, cfg.Postgres.ConnectionURI)
	if err != nil {
		log.Error(ctx, "cannot connect to database", sl.Err(err))
		os.Exit(1)
	}
	defer pgxPool.Close()

	usersRepo := users.NewInMemoryRepo()

	firstUser := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	secondUser := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	usersRepo.Populate(map[uuid.UUID]string{
		firstUser:  "first@test.com",
		secondUser: "second@test.com",
	})

	mailOptions := []mail.Option{
		mail.WithPort(cfg.Smtp.Port),
		mail.WithUsername(cfg.Smtp.Username),
		mail.WithPassword(cfg.Smtp.Password),
	}
	if cfg.Smtp.TLS {
		mailOptions = append(mailOptions, mail.WithTLSPolicy(mail.TLSMandatory))
	} else {
		mailOptions = append(mailOptions, mail.WithTLSPolicy(mail.NoTLS))
	}
	mailClient, err := mail.NewClient(
		cfg.Smtp.Host,
		mailOptions...,
	)
	if err != nil {
		log.Error(ctx, "cannot create mail client", sl.Err(err))
		os.Exit(1)
	}
	emailSender := email_messages_sender.New(
		usersRepo,
		mailClient,
		cfg.Smtp.From,
	)

	router := NewRouter(
		log,
		pgxPool,
		[]byte(cfg.Auth.Secret),
		usersRepo,
		emailSender,
	)

	srv := http.Server{
		Addr:    cfg.Server.Address,
		Handler: router,
	}

	go func() {
		log.Info(ctx, "starting server", slog.String("address", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(ctx, "cannot start server", sl.Err(err))
			os.Exit(1)
		}
	}()

	log.Info(ctx, "press CTRL-C to exit")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	s := <-stop
	log.Info(ctx, "signal received", slog.String("signal", s.String()))

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error(ctx, "force shutdown", sl.Err(err))
		os.Exit(1)
	}
	log.Info(ctx, "graceful shutdown")
}
