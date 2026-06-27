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
	"github.com/zulkhair/pustaka/backend/internal/pkg/jwt"
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

func (s *Service) issueTokens(ctx context.Context, u domain.User) (Tokens, error) {
	access, err := jwt.GenerateAccess(u.ID, u.Role, s.cfg.JWTSecret, s.cfg.AccessTTL)
	if err != nil {
		return Tokens{}, fmt.Errorf("generate access token: %w", err)
	}
	refresh, err := jwt.GenerateRefreshToken()
	if err != nil {
		return Tokens{}, fmt.Errorf("generate refresh token: %w", err)
	}
	_, err = s.store.CreateSession(ctx, domain.CreateSessionParams{
		ID:               uuid.NewString(),
		UserID:           u.ID,
		RefreshTokenHash: hash.HashRefreshToken(refresh),
		ExpiresAt:        time.Now().Add(s.cfg.RefreshTTL),
	})
	if err != nil {
		return Tokens{}, fmt.Errorf("create session: %w", err)
	}
	return Tokens{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int(s.cfg.AccessTTL.Seconds()),
	}, nil
}

func (s *Service) VerifyEmail(ctx context.Context, in VerifyInput) (Tokens, error) {
	email := normalizeEmail(in.Email)

	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return Tokens{}, domain.ErrInvalidCode
		}
		return Tokens{}, err
	}

	ev, err := s.store.GetActiveEmailVerification(ctx, user.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return Tokens{}, domain.ErrInvalidCode
		}
		return Tokens{}, err
	}

	if time.Now().After(ev.ExpiresAt) {
		return Tokens{}, domain.ErrCodeExpired
	}

	if !hash.CheckCode(ev.CodeHash, in.Code) {
		// Atomic increment-then-compare: the UPDATE returns the new count.
		attempts, incErr := s.store.IncrementVerificationAttempts(ctx, ev.ID)
		if incErr != nil {
			return Tokens{}, incErr
		}
		if attempts >= s.cfg.MaxAttempts {
			return Tokens{}, domain.ErrTooManyAttempts
		}
		return Tokens{}, domain.ErrInvalidCode
	}

	var tokens Tokens
	txErr := s.store.ExecTx(ctx, func(st domain.Store) error {
		if err := st.SetUserEmailVerified(ctx, user.ID); err != nil {
			return err
		}
		if err := st.ConsumeEmailVerification(ctx, ev.ID); err != nil {
			return err
		}
		access, err := jwt.GenerateAccess(user.ID, user.Role, s.cfg.JWTSecret, s.cfg.AccessTTL)
		if err != nil {
			return fmt.Errorf("generate access token: %w", err)
		}
		refresh, err := jwt.GenerateRefreshToken()
		if err != nil {
			return fmt.Errorf("generate refresh token: %w", err)
		}
		if _, err := st.CreateSession(ctx, domain.CreateSessionParams{
			ID:               uuid.NewString(),
			UserID:           user.ID,
			RefreshTokenHash: hash.HashRefreshToken(refresh),
			ExpiresAt:        time.Now().Add(s.cfg.RefreshTTL),
		}); err != nil {
			return err
		}
		tokens = Tokens{
			AccessToken:  access,
			RefreshToken: refresh,
			ExpiresIn:    int(s.cfg.AccessTTL.Seconds()),
		}
		return nil
	})
	if txErr != nil {
		return Tokens{}, txErr
	}
	return tokens, nil
}
