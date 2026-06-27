package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zulkhair/pustaka/backend/internal/adapter/store/sqlc"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

// Store implements domain.Store over pgx + sqlc.
type Store struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, q: sqlc.New(pool)}
}

// Pool returns the underlying pgx pool (used by the shared test harness).
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// withQueries builds a Store bound to an existing sqlc.Queries (used inside a tx).
func (s *Store) withQueries(q *sqlc.Queries) *Store {
	return &Store{pool: s.pool, q: q}
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrConflict
	}
	return err
}

func (s *Store) ExecTx(ctx context.Context, fn func(domain.Store) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(s.withQueries(s.q.WithTx(tx))); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func toUser(r sqlc.WebUser) domain.User {
	return domain.User{
		ID:            r.ID,
		Username:      r.Username,
		Email:         r.Email,
		PasswordHash:  r.PasswordHash,
		Role:          r.Role,
		EmailVerified: r.EmailVerified,
		CreatedAt:     r.CreatedAt.Time,
	}
}

func toVerification(r sqlc.EmailVerification) domain.EmailVerification {
	v := domain.EmailVerification{
		ID:        r.ID,
		UserID:    r.UserID,
		CodeHash:  r.CodeHash,
		ExpiresAt: r.ExpiresAt.Time,
		Attempts:  int(r.Attempts),
		CreatedAt: r.CreatedAt.Time,
	}
	if r.ConsumedAt.Valid {
		t := r.ConsumedAt.Time
		v.ConsumedAt = &t
	}
	return v
}

func toSession(r sqlc.Session) domain.Session {
	sess := domain.Session{
		ID:               r.ID,
		UserID:           r.UserID,
		RefreshTokenHash: r.RefreshTokenHash,
		ExpiresAt:        r.ExpiresAt.Time,
		CreatedAt:        r.CreatedAt.Time,
	}
	if r.RevokedAt.Valid {
		t := r.RevokedAt.Time
		sess.RevokedAt = &t
	}
	return sess
}

func tstamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func (s *Store) CreateUser(ctx context.Context, p domain.CreateUserParams) (domain.User, error) {
	r, err := s.q.CreateUser(ctx, sqlc.CreateUserParams{
		ID:           p.ID,
		Username:     p.Username,
		Email:        p.Email,
		PasswordHash: p.PasswordHash,
		Role:         p.Role,
	})
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return toUser(r), nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	r, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return toUser(r), nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
	r, err := s.q.GetUserByUsername(ctx, username)
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return toUser(r), nil
}

func (s *Store) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	r, err := s.q.GetUserByID(ctx, id)
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return toUser(r), nil
}

func (s *Store) SetUserEmailVerified(ctx context.Context, id string) error {
	return mapErr(s.q.SetUserEmailVerified(ctx, id))
}

func (s *Store) CreateEmailVerification(ctx context.Context, p domain.CreateEmailVerificationParams) (domain.EmailVerification, error) {
	r, err := s.q.CreateEmailVerification(ctx, sqlc.CreateEmailVerificationParams{
		ID:        p.ID,
		UserID:    p.UserID,
		CodeHash:  p.CodeHash,
		ExpiresAt: tstamp(p.ExpiresAt),
	})
	if err != nil {
		return domain.EmailVerification{}, mapErr(err)
	}
	return toVerification(r), nil
}

func (s *Store) GetActiveEmailVerification(ctx context.Context, userID string) (domain.EmailVerification, error) {
	r, err := s.q.GetActiveEmailVerification(ctx, userID)
	if err != nil {
		return domain.EmailVerification{}, mapErr(err)
	}
	return toVerification(r), nil
}

func (s *Store) IncrementVerificationAttempts(ctx context.Context, id string) (int, error) {
	n, err := s.q.IncrementVerificationAttempts(ctx, id)
	if err != nil {
		return 0, mapErr(err)
	}
	return int(n), nil
}

func (s *Store) ConsumeEmailVerification(ctx context.Context, id string) error {
	return mapErr(s.q.ConsumeEmailVerification(ctx, id))
}

func (s *Store) DeleteEmailVerificationsByUser(ctx context.Context, userID string) error {
	return mapErr(s.q.DeleteEmailVerificationsByUser(ctx, userID))
}

func (s *Store) CreateSession(ctx context.Context, p domain.CreateSessionParams) (domain.Session, error) {
	r, err := s.q.CreateSession(ctx, sqlc.CreateSessionParams{
		ID:               p.ID,
		UserID:           p.UserID,
		RefreshTokenHash: p.RefreshTokenHash,
		ExpiresAt:        tstamp(p.ExpiresAt),
	})
	if err != nil {
		return domain.Session{}, mapErr(err)
	}
	return toSession(r), nil
}

func (s *Store) GetSessionByTokenHash(ctx context.Context, hash string) (domain.Session, error) {
	r, err := s.q.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		return domain.Session{}, mapErr(err)
	}
	return toSession(r), nil
}

func (s *Store) RevokeSession(ctx context.Context, id string) error {
	return mapErr(s.q.RevokeSession(ctx, id))
}

func (s *Store) RevokeAllUserSessions(ctx context.Context, userID string) error {
	return mapErr(s.q.RevokeAllUserSessions(ctx, userID))
}

var _ domain.Store = (*Store)(nil)
