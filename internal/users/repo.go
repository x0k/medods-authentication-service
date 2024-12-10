package users

import (
	"context"
	"maps"

	"github.com/google/uuid"
	"github.com/x0k/medods-authentication-service/internal/shared"
)

type inMemoryRepo struct {
	users map[uuid.UUID]string
}

func NewInMemoryRepo() *inMemoryRepo {
	return &inMemoryRepo{
		users: make(map[uuid.UUID]string),
	}
}

func (r *inMemoryRepo) Populate(users map[uuid.UUID]string) {
	maps.Copy(r.users, users)
}

func (r *inMemoryRepo) UserExists(ctx context.Context, id uuid.UUID) (bool, error) {
	_, ok := r.users[id]
	return ok, nil
}

func (r *inMemoryRepo) EmailById(ctx context.Context, id uuid.UUID) (string, error) {
	email, ok := r.users[id]
	if ok {
		return email, nil
	}
	return "", shared.ErrNotFound
}
