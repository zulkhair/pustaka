package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/zulkhair/pustaka/backend/internal/config"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
)

type Service struct {
	store  domain.Store
	mailer domain.Mailer
	cfg    config.Config
}

func New(store domain.Store, mailer domain.Mailer, cfg config.Config) *Service {
	return &Service{store: store, mailer: mailer, cfg: cfg}
}

type RegisterInput struct {
	Username string
	Email    string
	Password string
}

type VerifyInput struct {
	Email string
	Code  string
}

type LoginInput struct {
	Identifier string
	Password   string
}

type Tokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *Service) Register(ctx context.Context, in RegisterInput) error {
	username := strings.TrimSpace(in.Username)
	email := normalizeEmail(in.Email)
	if username == "" || email == "" || in.Password == "" {
		return fmt.Errorf("%w: missing required field", domain.ErrValidation)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("%w: invalid email", domain.ErrValidation)
	}
	if len(in.Password) < 8 {
		return fmt.Errorf("%w: password too short", domain.ErrValidation)
	}

	pwHash, err := hash.HashPassword(in.Password, s.cfg.BcryptCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	code, err := hash.GenerateNumericCode(6)
	if err != nil {
		return fmt.Errorf("generate code: %w", err)
	}
	codeHash, err := hash.HashCode(code, s.cfg.BcryptCost)
	if err != nil {
		return fmt.Errorf("hash code: %w", err)
	}

	txErr := s.store.ExecTx(ctx, func(st domain.Store) error {
		if _, err := st.GetUserByEmail(ctx, email); err == nil {
			return domain.ErrConflict
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		if _, err := st.GetUserByUsername(ctx, username); err == nil {
			return domain.ErrConflict
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}

		user, err := st.CreateUser(ctx, domain.CreateUserParams{
			ID:           uuid.NewString(),
			Username:     username,
			Email:        email,
			PasswordHash: pwHash,
			Role:         domain.RoleUser,
		})
		if err != nil {
			return err
		}

		_, err = st.CreateEmailVerification(ctx, domain.CreateEmailVerificationParams{
			ID:        uuid.NewString(),
			UserID:    user.ID,
			CodeHash:  codeHash,
			ExpiresAt: time.Now().Add(s.cfg.CodeTTL),
		})
		return err
	})
	if txErr != nil {
		return txErr
	}

	if err := s.mailer.SendVerificationCode(ctx, email, code); err != nil {
		return fmt.Errorf("send verification code: %w", err)
	}
	return nil
}
