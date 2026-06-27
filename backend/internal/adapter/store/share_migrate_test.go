package store_test

import (
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
)

func TestMigrationDocumentShare(t *testing.T) {
	conn := startPostgres(t)
	require.NoError(t, store.RunMigrations(conn))

	db, err := sql.Open("pgx", conn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	var tbl string
	require.NoError(t, db.QueryRow(
		`SELECT table_name FROM information_schema.tables WHERE table_name = 'document_share'`,
	).Scan(&tbl))
	require.Equal(t, "document_share", tbl)

	for _, col := range []string{
		"id", "document_id", "shared_with_user_id", "permission", "created_at",
	} {
		var n int
		require.NoError(t, db.QueryRow(
			`SELECT count(*) FROM information_schema.columns WHERE table_name = 'document_share' AND column_name = $1`, col,
		).Scan(&n))
		require.Equal(t, 1, n, "column document_share.%s should exist", col)
	}

	var checks int
	require.NoError(t, db.QueryRow(
		`SELECT count(*) FROM information_schema.check_constraints
		 WHERE constraint_schema = 'public' AND check_clause ILIKE '%permission%'`,
	).Scan(&checks))
	require.GreaterOrEqual(t, checks, 1, "permission CHECK should exist")

	var uniques int
	require.NoError(t, db.QueryRow(
		`SELECT count(*) FROM information_schema.table_constraints
		 WHERE table_name = 'document_share' AND constraint_type = 'UNIQUE'`,
	).Scan(&uniques))
	require.GreaterOrEqual(t, uniques, 1, "unique(document_id, shared_with_user_id) should exist")

	for _, idx := range []string{"idx_document_share_doc", "idx_document_share_user"} {
		var name string
		require.NoError(t, db.QueryRow(
			`SELECT indexname FROM pg_indexes WHERE indexname = $1`, idx,
		).Scan(&name), "index %s should exist", idx)
		require.Equal(t, idx, name)
	}
}
