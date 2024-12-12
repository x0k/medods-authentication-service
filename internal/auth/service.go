package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/x0k/medods-authentication-service/internal/lib/logger"
	"github.com/x0k/medods-authentication-service/internal/lib/logger/sl"
	"github.com/x0k/medods-authentication-service/internal/lib/unit_of_work"
	"github.com/x0k/medods-authentication-service/internal/shared"
	"golang.org/x/crypto/bcrypt"
)

var ErrFailedToAuthenticate = errors.New("failed to authenticate")
var ErrFailedToIssueTokens = errors.New("failed to issue tokens")
var ErrFailedToRefreshTokens = errors.New("failed to refresh tokens")

type DeviceId = [32]byte

type RefreshTokensRepository[T any] interface {
	UpsertTokenHash(ctx context.Context, userId uuid.UUID, deviceId DeviceId, tokenHash []byte) error
	TokenHash(
		ctx context.Context,
		uow unit_of_work.UnitOfWork[T],
		userId uuid.UUID,
		deviceId DeviceId,
	) ([]byte, error)
	UpdateTokenHash(
		ctx context.Context,
		uow unit_of_work.UnitOfWork[T],
		userId uuid.UUID,
		oldDeviceId DeviceId,
		newDeviceId DeviceId,
		tokenHash []byte,
	) error
}

type UsersRepository interface {
	UserExists(ctx context.Context, id uuid.UUID) (bool, error)
}

type MessagesSender interface {
	SendWarning(ctx context.Context, userId uuid.UUID, message string) error
}

type service[T any] struct {
	log               *logger.Logger
	secret            []byte
	usersRepo         UsersRepository
	refreshTokensRepo RefreshTokensRepository[T]
	sender            MessagesSender
	uowFactory        unit_of_work.Factory[T]
}

type tokens struct {
	accessToken           string
	refreshToken          string
	hashOfAccessTokenHash []byte
}

func newService[T any](
	log *logger.Logger,
	secret []byte,
	usersRepo UsersRepository,
	refreshTokensRepo RefreshTokensRepository[T],
	sender MessagesSender,
	uowFactory unit_of_work.Factory[T],
) *service[T] {
	return &service[T]{
		log:               log,
		secret:            secret,
		usersRepo:         usersRepo,
		refreshTokensRepo: refreshTokensRepo,
		sender:            sender,
		uowFactory:        uowFactory,
	}
}

func (s *service[T]) IssueTokens(ctx context.Context, userId uuid.UUID, ipAddress string) (string, string, *shared.DomainError) {
	if err := s.userExists(ctx, userId); err != nil {
		err.Err = fmt.Errorf("%w: %s", ErrFailedToIssueTokens, err.Err)
		return "", "", err
	}
	tokens, err := s.issueTokens(userId, ipAddress)
	if err != nil {
		return "", "", err
	}
	// В задании отсутствует информация об ограничениях на количество токенов для одного пользователя.
	// Будем считать что один пользователь может иметь по одному Refresh токену на устройство.
	// За неимением другой информации в качестве id устройства будем использовать ip-адрес.
	deviceId := sha256.Sum256([]byte(ipAddress))
	if err := s.refreshTokensRepo.UpsertTokenHash(ctx, userId, deviceId, tokens.hashOfAccessTokenHash); err != nil {
		return "", "", shared.NewUnexpectedError(
			fmt.Errorf("%w: save token hash: %s", ErrFailedToIssueTokens, err),
			"failed to persist token",
		)
	}
	return tokens.accessToken, tokens.refreshToken, nil
}

func (s *service[T]) Refresh(
	ctx context.Context,
	accessTokenString string,
	refreshTokenString string,
	ipAddress string,
) (string, string, *shared.DomainError) {
	accessToken, err := jwt.Parse(accessTokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method: %s", ErrFailedToRefreshTokens, t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return "", "", shared.NewDomainError(
			fmt.Errorf("%w: access token: %s", ErrFailedToRefreshTokens, err),
			"invalid access token",
		)
	}
	decodedRefreshTokenString, err := base64.URLEncoding.DecodeString(refreshTokenString)
	if err != nil {
		return "", "", shared.NewDomainError(
			fmt.Errorf("%w: refresh token: %s", ErrFailedToRefreshTokens, err),
			"invalid refresh token encoding",
		)
	}
	refreshToken, err := jwt.Parse(string(decodedRefreshTokenString), func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method: %s", ErrFailedToRefreshTokens, t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return "", "", shared.NewDomainError(
			fmt.Errorf("%w: refresh token: %s", ErrFailedToRefreshTokens, err),
			"invalid refresh token",
		)
	}
	accessTokenClaims := accessToken.Claims.(jwt.MapClaims)
	accessTokenHash := sha256.Sum256([]byte(accessTokenString))
	accessTokenHashSlice := accessTokenHash[:]
	refreshTokenClaims := refreshToken.Claims.(jwt.MapClaims)
	base64EncodedAccessTokenHash := refreshTokenClaims["sub"].(string)
	accessTokenHashFromRefreshToken, err := base64.URLEncoding.DecodeString(base64EncodedAccessTokenHash)
	if err != nil {
		return "", "", shared.NewUnexpectedError(
			fmt.Errorf("%w: access token hash: %s", ErrFailedToRefreshTokens, err),
			"failed to decode access token",
		)
	}
	if !bytes.Equal(accessTokenHashSlice, accessTokenHashFromRefreshToken) {
		return "", "", shared.NewDomainError(
			fmt.Errorf("%w: tokens mismatch", ErrFailedToRefreshTokens),
			"tokens mismatch",
		)
	}
	userId := uuid.MustParse(accessTokenClaims["sub"].(string))
	if err := s.userExists(ctx, userId); err != nil {
		err.Err = fmt.Errorf("%w: %s", ErrFailedToRefreshTokens, err.Err)
		return "", "", err
	}
	oldIpAddress := accessTokenClaims["ip"].(string)
	oldDeviceId := sha256.Sum256([]byte(oldIpAddress))
	uow, err := s.uowFactory(ctx)
	if err != nil {
		return "", "", shared.NewUnexpectedError(
			fmt.Errorf("%w: create unit of work: %s", ErrFailedToRefreshTokens, err),
			"failed to persist token",
		)
	}
	defer func() {
		if err := uow.Rollback(ctx); err != nil {
			s.log.Error(ctx, "failed to rollback unit of work", sl.Err(err))
		}
	}()
	tokenHash, err := s.refreshTokensRepo.TokenHash(ctx, uow, userId, oldDeviceId)
	if errors.Is(err, shared.ErrNotFound) {
		s.log.Warn(
			ctx,
			"refresh token reuse attempt",
			slog.String("user_id", userId.String()),
			slog.String("ip", ipAddress),
		)
		return "", "", shared.NewDomainError(
			fmt.Errorf("%w: token hash not found", ErrFailedToRefreshTokens),
			"invalid refresh token",
		)
	}
	if err != nil {
		return "", "", shared.NewUnexpectedError(
			fmt.Errorf("%w: get token hash: %s", ErrFailedToRefreshTokens, err),
			"failed to check refresh token",
		)
	}
	err = bcrypt.CompareHashAndPassword(tokenHash, accessTokenHashSlice)
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		s.log.Warn(
			ctx,
			"stale tokens pair reuse attempt",
			slog.String("user_id", userId.String()),
			slog.String("ip", ipAddress),
		)
		return "", "", shared.NewDomainError(
			fmt.Errorf("%w: compare hash: %s", ErrFailedToRefreshTokens, err),
			"invalid refresh token",
		)
	}
	if err != nil {
		return "", "", shared.NewUnexpectedError(
			fmt.Errorf("%w: compare hash: %s", ErrFailedToRefreshTokens, err),
			"failed to check refresh token",
		)
	}
	tokens, dErr := s.issueTokens(userId, ipAddress)
	if dErr != nil {
		dErr.Err = fmt.Errorf("%w: %s", ErrFailedToRefreshTokens, dErr.Err)
		return "", "", dErr
	}
	var message string
	if oldIpAddress != ipAddress {
		s.log.Warn(
			ctx,
			"ip mismatch",
			slog.String("user_id", userId.String()),
			slog.String("old_ip", oldIpAddress),
			slog.String("new_ip", ipAddress),
		)
		newDeviceId := sha256.Sum256([]byte(ipAddress))
		if err := s.refreshTokensRepo.UpdateTokenHash(
			ctx,
			uow,
			userId,
			oldDeviceId,
			newDeviceId,
			tokens.hashOfAccessTokenHash,
		); err != nil {
			return "", "", shared.NewUnexpectedError(
				fmt.Errorf("%w: replace token hash: %s", ErrFailedToRefreshTokens, err),
				"failed to persist token",
			)
		}
		message = fmt.Sprintf("ip mismatch: %s != %s", oldIpAddress, ipAddress)
	} else if err = s.refreshTokensRepo.UpdateTokenHash(
		ctx,
		uow,
		userId,
		oldDeviceId,
		oldDeviceId,
		tokens.hashOfAccessTokenHash,
	); err != nil {
		return "", "", shared.NewUnexpectedError(
			fmt.Errorf("%w: save token hash: %s", ErrFailedToRefreshTokens, err),
			"failed to persist token",
		)
	}
	if err = uow.Commit(ctx); err != nil {
		return "", "", shared.NewUnexpectedError(
			fmt.Errorf("%w: commit unit of work: %s", ErrFailedToRefreshTokens, err),
			"failed to persist token",
		)
	}
	if message != "" {
		if err = s.sender.SendWarning(ctx, userId, message); err != nil {
			s.log.Error(
				ctx,
				"failed to send warning",
				slog.String("user_id", userId.String()),
				slog.String("message", message), sl.Err(err),
			)
		}
	}
	return tokens.accessToken, tokens.refreshToken, nil
}

func (s *service[T]) userExists(
	ctx context.Context,
	userId uuid.UUID,
) *shared.DomainError {
	exists, err := s.usersRepo.UserExists(ctx, userId)
	if err != nil {
		return shared.NewUnexpectedError(
			fmt.Errorf("%w: check if user exists: %s", ErrFailedToAuthenticate, err),
			"failed to get user info",
		)
	}
	if !exists {
		return shared.NewDomainError(
			fmt.Errorf("%w: user not found", ErrFailedToAuthenticate),
			"invalid credentials",
		)
	}
	return nil
}

func (s *service[T]) issueTokens(
	userId uuid.UUID,
	ipAddress string,
) (tokens, *shared.DomainError) {
	// > Access токен тип JWT, алгоритм SHA512
	// Наверно имелся в виду `HMAC-SHA512` (`HS512`)
	t := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"sub": userId,
		"ip":  ipAddress,
		// Access токен должен быть уникальным, иначе один Access токен
		// будет подходить к нескольким Refresh токенам и наоборот
		"jti": uuid.New().String(),
	})
	accessToken, err := t.SignedString(s.secret)
	if err != nil {
		return tokens{}, shared.NewUnexpectedError(
			fmt.Errorf("%w: access token: %s", ErrFailedToIssueTokens, err),
			"failed to sign access token",
		)
	}
	accessTokenHash := sha256.Sum256([]byte(accessToken))
	accessTokenHashSlice := accessTokenHash[:]
	t = jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"sub": base64.URLEncoding.EncodeToString(accessTokenHashSlice),
		// > Payload токенов должен содержать сведения об ip адресе клиента
		// Не понятно зачем оно тут, может опечатка или пропущено слово `Access`?
		"ip": ipAddress,
	})
	// > Refresh токен ... должен быть защищен от изменения на стороне клиента
	// Тут может быть речь про HttpOnly Secure Lax/Strict Cookie, но т.к. нет
	// информации о клиентах, то будем считать что речь про JWS/JWE
	refreshToken, err := t.SignedString(s.secret)
	if err != nil {
		return tokens{}, shared.NewUnexpectedError(
			fmt.Errorf("%w: refresh token: %s", ErrFailedToIssueTokens, err),
			"failed to sign refresh token",
		)
	}
	// Тут нет разницы между хешем Access токена и хешем Access токена c подписью.
	hashOfAccessTokenHash, err := bcrypt.GenerateFromPassword(accessTokenHashSlice, bcrypt.DefaultCost)
	if err != nil {
		return tokens{}, shared.NewUnexpectedError(
			fmt.Errorf("%w: access token hash: %s", ErrFailedToIssueTokens, err),
			"failed to generate refresh token",
		)
	}
	// > Refresh токен тип произвольный, формат передачи base64
	base64EncodedRefreshToken := base64.URLEncoding.EncodeToString([]byte(refreshToken))
	return tokens{
		accessToken:           accessToken,
		refreshToken:          base64EncodedRefreshToken,
		hashOfAccessTokenHash: hashOfAccessTokenHash,
	}, nil
}
