package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

// newShareStore boots a migrated testcontainers Postgres and returns a *Store.
func newShareStore(t *testing.T) *Store {
	t.Helper()
	dsn := startPostgres(t) // package store, db_test.go (Plan 1)
	require.NoError(t, RunMigrations(dsn))
	ctx := context.Background()
	pool, err := OpenPool(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return New(pool)
}

func seedShareUser(t *testing.T, st *Store, username, email string) domain.User {
	t.Helper()
	u, err := st.CreateUser(context.Background(), domain.CreateUserParams{
		ID: uuid.NewString(), Username: username, Email: email,
		PasswordHash: "x", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	return u
}

func seedShareDoc(t *testing.T, st *Store, ownerID, title string) domain.Document {
	t.Helper()
	d, err := st.CreateDocument(context.Background(), domain.CreateDocumentParams{
		ID: uuid.NewString(), UserID: ownerID, Title: title, Mode: "text",
	})
	require.NoError(t, err)
	return d
}

func TestShareStoreCRUD(t *testing.T) {
	st := newShareStore(t)
	ctx := context.Background()

	owner := seedShareUser(t, st, "owner", "owner@example.com")
	sharee := seedShareUser(t, st, "sharee", "sharee@example.com")
	doc := seedShareDoc(t, st, owner.ID, "Shared Doc")

	share, err := st.CreateShare(ctx, domain.CreateShareParams{
		ID: uuid.NewString(), DocumentID: doc.ID,
		SharedWithUserID: sharee.ID, Permission: domain.PermissionViewer,
	})
	require.NoError(t, err)
	require.Equal(t, domain.PermissionViewer, share.Permission)

	got, err := st.GetShare(ctx, doc.ID, sharee.ID)
	require.NoError(t, err)
	require.Equal(t, share.ID, got.ID)

	// CreateShare again (idempotent upsert) does not error
	_, err = st.CreateShare(ctx, domain.CreateShareParams{
		ID: uuid.NewString(), DocumentID: doc.ID,
		SharedWithUserID: sharee.ID, Permission: domain.PermissionViewer,
	})
	require.NoError(t, err)

	shares, err := st.ListSharesForDocument(ctx, doc.ID)
	require.NoError(t, err)
	require.Len(t, shares, 1)

	sharedDocs, err := st.ListDocumentsSharedWith(ctx, sharee.ID)
	require.NoError(t, err)
	require.Len(t, sharedDocs, 1)
	require.Equal(t, doc.ID, sharedDocs[0].ID)

	require.NoError(t, st.DeleteShare(ctx, doc.ID, sharee.ID))
	_, err = st.GetShare(ctx, doc.ID, sharee.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)

	sharedDocs, err = st.ListDocumentsSharedWith(ctx, sharee.ID)
	require.NoError(t, err)
	require.Empty(t, sharedDocs)
}
