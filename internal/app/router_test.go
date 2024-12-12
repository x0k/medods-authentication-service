package app_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gavv/httpexpect/v2"
	"github.com/google/uuid"
	"github.com/x0k/medods-authentication-service/internal/app"
	"github.com/x0k/medods-authentication-service/internal/lib/logger"
	email_messages_sender "github.com/x0k/medods-authentication-service/internal/messages_sender/email"
	"github.com/x0k/medods-authentication-service/internal/testutils"
	"github.com/x0k/medods-authentication-service/internal/users"
)

func TestRouter(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	t.Cleanup(func() {
		if t.Failed() {
			t.Log("log:", buf.String())
		}
	})
	log := logger.New(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	pgxPool := testutils.SetupPgxPool(ctx, log.Logger, t)
	secret := []byte("secret")
	usersRepo := users.NewInMemoryRepo()
	userId := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	userEmail := "user@test.com"
	senderEmail := "<admin@auth.com>"
	usersRepo.Populate(map[uuid.UUID]string{
		userId: userEmail,
	})
	mailClient, mailApiClient := testutils.SetupMailClient(ctx, t)
	emailSender := email_messages_sender.New(
		usersRepo,
		mailClient,
		senderEmail,
	)

	router := app.NewRouter(log, pgxPool, secret, usersRepo, emailSender)

	server := httptest.NewServer(router)
	defer server.Close()
	e := httpexpect.Default(t, server.URL)

	resp := e.POST("/auth/login").
		WithQuery("GUID", userId.String()).
		Expect().
		Status(http.StatusOK).
		JSON().Object()

	resp.Keys().ContainsOnly("accessToken", "refreshToken")
	accessToken := resp.Value("accessToken").String().Raw()
	refreshToken := resp.Value("refreshToken").String().Raw()

	resp = e.POST("/auth/refresh").
		WithJSON(map[string]string{
			"accessToken":  accessToken,
			"refreshToken": refreshToken,
		}).
		Expect().
		Status(http.StatusOK).
		JSON().Object()
	resp.Keys().ContainsOnly("accessToken", "refreshToken")
	accessToken = resp.Value("accessToken").String().Raw()
	refreshToken = resp.Value("refreshToken").String().Raw()

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.RemoteAddr = "127.0.0.2:8080"
		router.ServeHTTP(w, r)
	}))
	defer server.Close()
	e = httpexpect.Default(t, server.URL)

	e.POST("/auth/refresh").
		WithJSON(map[string]string{
			"accessToken":  accessToken,
			"refreshToken": refreshToken,
		}).
		Expect().
		Status(http.StatusOK).
		JSON().Object().Keys().ContainsOnly("accessToken", "refreshToken")

	messages, err := mailApiClient.ListMailbox(userEmail)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].From != senderEmail {
		t.Errorf("expected sender %s, got %s", senderEmail, messages[0].From)
	}
}
