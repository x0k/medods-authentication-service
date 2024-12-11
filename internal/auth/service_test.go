package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	mock "github.com/stretchr/testify/mock"
	"github.com/x0k/medods-authentication-service/internal/lib/logger"
	unit_of_work "github.com/x0k/medods-authentication-service/internal/lib/unit_of_work"
	"github.com/x0k/medods-authentication-service/internal/shared"
)

type serviceMocks struct {
	users         *MockUsersRepository
	refreshTokens *MockRefreshTokensRepository[any]
	sender        *MockMessagesSender
	uowFactory    *unit_of_work.MockFactory[any]
	uow           *unit_of_work.MockUnitOfWork[any]
}

func newTestService(t *testing.T, secret []byte, setup func(serviceMocks)) *service[any] {
	var buf bytes.Buffer
	log := logger.New(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	users := NewMockUsersRepository(t)
	refreshTokens := NewMockRefreshTokensRepository[any](t)
	sender := NewMockMessagesSender(t)
	uowFactory := unit_of_work.NewMockFactory[any](t)
	uow := unit_of_work.NewMockUnitOfWork[any](t)
	setup(serviceMocks{
		users:         users,
		refreshTokens: refreshTokens,
		sender:        sender,
		uowFactory:    uowFactory,
		uow:           uow,
	})
	return newService(
		log,
		secret,
		users,
		refreshTokens,
		sender,
		uowFactory.Execute,
	)
}

func TestServiceIssueTokens(t *testing.T) {
	secret := []byte("secret")
	userId := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	userIpAddress := "127.0.0.1"
	userDeviceId := sha256.Sum256([]byte(userIpAddress))

	cases := []struct {
		name      string
		service   *service[any]
		userId    uuid.UUID
		ipAddress string
		err       *shared.DomainError
	}{
		{
			name: "should return error if user does not exist",
			service: newTestService(t, secret, func(sm serviceMocks) {
				sm.users.EXPECT().UserExists(mock.Anything, mock.Anything).Return(false, nil)
			}),
			err: shared.NewDomainError(
				ErrFailedToIssueTokens,
				"",
			),
		},
		{
			name: "should issue tokens for the identified user",
			service: newTestService(t, secret, func(sm serviceMocks) {
				sm.users.EXPECT().UserExists(mock.Anything, userId).Return(true, nil)
				sm.refreshTokens.EXPECT().
					UpsertTokenHash(mock.Anything, userId, userDeviceId, mock.Anything).
					Return(nil)
			}),
			userId:    userId,
			ipAddress: userIpAddress,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			accessToken, refreshToken, dErr := c.service.IssueTokens(
				context.Background(),
				c.userId,
				c.ipAddress,
			)
			if dErr != nil {
				if c.err == nil ||
					!errors.Is(dErr.Err, c.err.Err) ||
					dErr.Expected != c.err.Expected ||
					(c.err.Msg != "" && dErr.Msg != c.err.Msg) {
					t.Fatalf("unexpected error: %v", dErr)
				}
				return
			}
			if _, err := jwt.Parse(accessToken, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %s", t.Header["alg"])
				}
				return secret, nil
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := base64.URLEncoding.DecodeString(refreshToken); err != nil {
				t.Fatalf("failed to decode refresh token: %s", err)
			}
		})
	}
}
