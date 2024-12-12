package testutils

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/inbucket/inbucket/pkg/rest/client"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/inbucket"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/wneessen/go-mail"
	pgx_adapter "github.com/x0k/medods-authentication-service/internal/adapters/pgx"
)

func SetupPgxPool(ctx context.Context, log *slog.Logger, t testing.TB) *pgxpool.Pool {
	pgContainer, err := postgres.Run(
		ctx, "postgres:17.2-alpine3.20",
		postgres.WithDatabase("auth"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		t.Fatal(err)
	}
	testcontainers.CleanupContainer(t, pgContainer)

	uri, err := pgContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := pgx_adapter.Migrate(ctx, log, uri, "file://../../migrations"); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func SetupMailClient(ctx context.Context, t testing.TB) (*mail.Client, *client.Client) {
	inbucketContainer, err := inbucket.Run(ctx, "inbucket/inbucket:3.1.0-beta3")
	if err != nil {
		t.Fatal(err)
	}
	testcontainers.CleanupContainer(t, inbucketContainer)

	smtpUrl, err := inbucketContainer.SmtpConnection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	hostAndPort := strings.Split(smtpUrl, ":")
	port, err := strconv.Atoi(hostAndPort[1])
	if err != nil {
		t.Fatal(err)
	}
	mailClient, err := mail.NewClient(
		hostAndPort[0],
		mail.WithPort(port),
		mail.WithTLSPolicy(mail.NoTLS),
	)
	if err != nil {
		t.Fatal(err)
	}
	apiUrl, err := inbucketContainer.WebInterface(ctx)
	if err != nil {
		t.Fatal(err)
	}
	apiClient, err := client.New(apiUrl)
	if err != nil {
		t.Fatal(err)
	}
	return mailClient, apiClient
}
