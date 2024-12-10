package email_messages_sender

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/wneessen/go-mail"
)

type UsersRepository interface {
	EmailById(ctx context.Context, id uuid.UUID) (string, error)
}

type Sender struct {
	repo   UsersRepository
	client *mail.Client
	sender string
}

func New(
	repo UsersRepository,
	client *mail.Client,
	sender string,
) *Sender {
	return &Sender{
		repo:   repo,
		client: client,
		sender: sender,
	}
}

func (s *Sender) SendWarning(ctx context.Context, userId uuid.UUID, message string) error {
	msg := mail.NewMsg()
	if err := msg.From(s.sender); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	email, err := s.repo.EmailById(ctx, userId)
	if err != nil {
		return fmt.Errorf("failed to get user email: %w", err)
	}

	if err := msg.To(email); err != nil {
		return fmt.Errorf("failed to set receiver: %w", err)
	}
	msg.Subject("Warning")
	msg.SetBodyString(mail.TypeTextPlain, message)
	return s.client.DialAndSendWithContext(ctx, msg)
}
