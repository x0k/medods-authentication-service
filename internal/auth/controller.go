package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/x0k/medods-authentication-service/internal/lib/httpx"
	"github.com/x0k/medods-authentication-service/internal/lib/logger"
	"github.com/x0k/medods-authentication-service/internal/lib/logger/sl"
	"github.com/x0k/medods-authentication-service/internal/shared"
)

var ErrInvalidGUID = errors.New("invalid GUID")

type AuthService interface {
	IssueTokens(ctx context.Context, userId uuid.UUID, ipAddress string) (string, string, *shared.DomainError)
	Refresh(ctx context.Context, accessToken string, refreshToken string, ipAddress string) (string, string, *shared.DomainError)
}

type controller struct {
	log         *logger.Logger
	authService AuthService
	decoder     *httpx.JsonBodyDecoder
}

func newController(
	log *logger.Logger,
	authService AuthService,
) *controller {
	return &controller{
		log:         log,
		authService: authService,
		decoder: &httpx.JsonBodyDecoder{
			MaxBytes:              1024,
			DisallowUnknownFields: true,
		},
	}
}

type tokensDTO struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// Первый маршрут выдает пару Access, Refresh токенов для пользователя
// с идентификатором (GUID) указанным в параметре запроса
func (c *controller) Login(w http.ResponseWriter, r *http.Request) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		c.badRequest(w, r, err, "failed to parse query")
		return
	}
	guidParameter := q.Get("GUID")
	userId, err := c.parseGUID(guidParameter)
	if err != nil {
		c.badRequest(w, r, err, "failed to parse GUID")
		return
	}
	accessToken, refreshToken, dErr := c.authService.IssueTokens(r.Context(), userId, r.RemoteAddr)
	if dErr != nil {
		c.domainError(w, r, dErr)
		return
	}
	c.json(w, r, tokensDTO{accessToken, refreshToken}, http.StatusOK)
}

func (c *controller) Refresh(w http.ResponseWriter, r *http.Request) {
	tokens, httpErr := httpx.JSONBody[tokensDTO](c.decoder, w, r)
	if httpErr != nil {
		http.Error(w, httpErr.Text, httpErr.Status)
		c.log.Debug(r.Context(), "failed to decode JSON", sl.Err(httpErr))
		return
	}
	accessToken, refreshToken, err := c.authService.Refresh(
		r.Context(),
		tokens.AccessToken,
		tokens.RefreshToken,
		r.RemoteAddr,
	)
	if err != nil {
		c.domainError(w, r, err)
		return
	}
	c.json(w, r, tokensDTO{accessToken, refreshToken}, http.StatusOK)
}

func (c *controller) parseGUID(guid string) (uuid.UUID, error) {
	if guid == "" {
		return uuid.Nil, fmt.Errorf("%w: empty", ErrInvalidGUID)
	}
	id, err := uuid.Parse(guid)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %s", ErrInvalidGUID, err)
	}
	return id, nil
}

func (c *controller) domainError(w http.ResponseWriter, r *http.Request, err *shared.DomainError) {
	if err.Expected {
		c.badRequest(w, r, err.Err, err.Msg)
	} else {
		c.serverError(w, r, err.Err, err.Msg)
	}
}

func (c *controller) badRequest(w http.ResponseWriter, r *http.Request, err error, msg string) {
	http.Error(w, msg, http.StatusBadRequest)
	c.log.Debug(r.Context(), msg, sl.Err(err))
}

func (c *controller) serverError(w http.ResponseWriter, r *http.Request, err error, msg string) {
	http.Error(w, msg, http.StatusInternalServerError)
	c.log.Error(r.Context(), msg, sl.Err(err))
}

func (c *controller) json(w http.ResponseWriter, r *http.Request, data any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		c.serverError(w, r, err, "failed to encode JSON")
	}
}
