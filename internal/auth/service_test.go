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
	"golang.org/x/crypto/bcrypt"
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
	if setup != nil {
		setup(serviceMocks{
			users:         users,
			refreshTokens: refreshTokens,
			sender:        sender,
			uowFactory:    uowFactory,
			uow:           uow,
		})
	}
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

func TestServiceRefresh(t *testing.T) {
	secret := []byte("secret")
	userId := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	userIpAddress := "127.0.0.1"
	userDeviceId := sha256.Sum256([]byte(userIpAddress))

	type testCase struct {
		name      string
		service   *service[any]
		tokens    tokens
		ipAddress string
		err       *shared.DomainError
	}

	newTestCase := func(
		name string,
		setup func(serviceMocks),
		update func(*testCase),
	) testCase {
		service := newTestService(t, secret, setup)
		tokens, err := service.issueTokens(userId, userIpAddress)
		if err != nil {
			t.Fatalf("failed to issue tokens: %v", err)
		}
		tc := testCase{
			name:      name,
			service:   service,
			tokens:    tokens,
			ipAddress: userIpAddress,
		}
		if update != nil {
			update(&tc)
		}
		return tc
	}

	cases := []testCase{
		{
			name:    "should return error if access token is invalid",
			service: newTestService(t, secret, nil),
			err: shared.NewDomainError(
				ErrFailedToRefreshTokens,
				"invalid access token",
			),
		},
		newTestCase(
			"should return error if refresh token is invalid",
			nil,
			func(tc *testCase) {
				tc.tokens.refreshToken = "not a base64 string"
				tc.err = shared.NewDomainError(
					ErrFailedToRefreshTokens,
					"invalid refresh token encoding",
				)
			},
		),
		newTestCase(
			"should return error if refresh token is invalid 2",
			nil,
			func(tc *testCase) {
				tc.tokens.refreshToken = base64.URLEncoding.EncodeToString(
					[]byte("invalid refresh token"),
				)
				tc.err = shared.NewDomainError(
					ErrFailedToRefreshTokens,
					"invalid refresh token",
				)
			},
		),
		newTestCase(
			"should return error if tokens are not related to each other",
			nil,
			func(tc *testCase) {
				tokens, err := tc.service.issueTokens(userId, userIpAddress)
				if err != nil {
					t.Fatalf("failed to issue tokens: %v", err)
				}
				tc.tokens.accessToken = tokens.accessToken
				tc.err = shared.NewDomainError(
					ErrFailedToRefreshTokens,
					"tokens mismatch",
				)
			},
		),
		newTestCase(
			"should return error if user is already removed",
			func(m serviceMocks) {
				m.users.EXPECT().UserExists(mock.Anything, userId).Return(false, nil)
			},
			func(tc *testCase) {
				tc.err = shared.NewDomainError(
					ErrFailedToRefreshTokens,
					"",
				)
			},
		),
		newTestCase(
			"should return error if there is no refresh for the user device",
			func(m serviceMocks) {
				m.users.EXPECT().UserExists(mock.Anything, userId).Return(true, nil)

				m.uowFactory.EXPECT().Execute(mock.Anything).Return(m.uow, nil)
				m.uow.EXPECT().Rollback(mock.Anything).Return(nil)

				m.refreshTokens.EXPECT().
					TokenHash(mock.Anything, m.uow, userId, userDeviceId).
					Return(nil, shared.ErrNotFound)
			},
			func(tc *testCase) {
				tc.err = shared.NewDomainError(
					ErrFailedToRefreshTokens,
					"invalid refresh token",
				)
			},
		),
		newTestCase(
			"should return error if stored refresh token is different",
			func(m serviceMocks) {
				m.users.EXPECT().UserExists(mock.Anything, userId).Return(true, nil)

				m.uowFactory.EXPECT().Execute(mock.Anything).Return(m.uow, nil)
				m.uow.EXPECT().Rollback(mock.Anything).Return(nil)

				hash, err := bcrypt.GenerateFromPassword(
					[]byte("different refresh token"),
					bcrypt.DefaultCost,
				)
				if err != nil {
					t.Fatalf("failed to generate hash: %v", err)
				}
				m.refreshTokens.EXPECT().
					TokenHash(mock.Anything, m.uow, userId, userDeviceId).
					Return(hash, nil)
			},
			func(tc *testCase) {
				tc.err = shared.NewDomainError(
					ErrFailedToRefreshTokens,
					"invalid refresh token",
				)
			},
		),
		newTestCase(
			"should return new tokens",
			nil,
			func(tc *testCase) {
				tc.service = newTestService(t, secret, func(m serviceMocks) {
					m.users.EXPECT().UserExists(mock.Anything, userId).Return(true, nil)

					m.uowFactory.EXPECT().Execute(mock.Anything).Return(m.uow, nil)
					m.uow.EXPECT().Rollback(mock.Anything).Return(nil)
					m.uow.EXPECT().Commit(mock.Anything).Return(nil)

					m.refreshTokens.EXPECT().
						TokenHash(mock.Anything, m.uow, userId, userDeviceId).
						RunAndReturn(func(
							ctx context.Context,
							uow unit_of_work.UnitOfWork[any],
							u uuid.UUID,
							b [32]byte,
						) ([]byte, error) {
							return tc.tokens.hashOfAccessTokenHash, nil
						})
					m.refreshTokens.EXPECT().
						UpdateTokenHash(
							mock.Anything,
							m.uow,
							userId,
							userDeviceId,
							userDeviceId,
							mock.AnythingOfType("[]uint8"),
						).
						Return(nil)
				})
			},
		),
		newTestCase(
			"should send warning if ip address is different",
			nil,
			func(tc *testCase) {
				tc.ipAddress = "127.0.0.2"
				tc.service = newTestService(t, secret, func(m serviceMocks) {
					m.users.EXPECT().UserExists(mock.Anything, userId).Return(true, nil)

					m.uowFactory.EXPECT().Execute(mock.Anything).Return(m.uow, nil)
					m.uow.EXPECT().Rollback(mock.Anything).Return(nil)
					m.uow.EXPECT().Commit(mock.Anything).Return(nil)

					m.refreshTokens.EXPECT().
						TokenHash(mock.Anything, m.uow, userId, userDeviceId).
						RunAndReturn(func(
							ctx context.Context,
							uow unit_of_work.UnitOfWork[any],
							u uuid.UUID,
							b [32]byte,
						) ([]byte, error) {
							return tc.tokens.hashOfAccessTokenHash, nil
						})
					m.refreshTokens.EXPECT().
						UpdateTokenHash(
							mock.Anything,
							m.uow,
							userId,
							userDeviceId,
							sha256.Sum256([]byte(tc.ipAddress)),
							mock.AnythingOfType("[]uint8"),
						).
						Return(nil)

					m.sender.EXPECT().
						SendWarning(mock.Anything, userId, mock.AnythingOfType("string")).
						Return(nil)
				})
			},
		),
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			accessToken, refreshToken, dErr := c.service.Refresh(
				context.Background(),
				c.tokens.accessToken,
				c.tokens.refreshToken,
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
