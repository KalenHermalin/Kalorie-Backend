package store

import (
	"context"
	"database/sql"
	"main/internal/models"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// resetDB empties every user-owned table now and again when the test ends.
// The exercise catalog and remind_user are not touched.
func resetDB(t *testing.T) {
	t.Helper()
	clearTables(t)
	t.Cleanup(func() { clearTables(t) })
}

// createUser signs a user up through the auth store and commits it.
func createUser(t *testing.T, firebaseID string, email *string) *models.User {
	t.Helper()
	authStore := NewPostgressAuthStore(testDB)
	var user *models.User
	err := authStore.WithTx(context.Background(), func(tx *sql.Tx) error {
		var err error
		user, err = authStore.UpsertUserWithAuth(context.Background(), tx, &models.AuthPayload{ID: firebaseID, Email: email})
		return err
	})
	require.NoError(t, err)
	return user
}

// inUserTx runs fn in a user-store transaction and returns its error. The transaction commits only if fn returns nil,
// so a test that expects a store error should return it from fn and assert on the result.
func inUserTx(fn func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error) error {
	s := NewPostgressUserStore(testDB)
	ctx := context.Background()
	return s.WithTx(ctx, func(tx *sql.Tx) error { return fn(ctx, s, tx) })
}

func strPtr(s string) *string { return &s }

func newID() string { return uuid.NewString() }

// baseTime is a fixed point every test builds its timestamps from, already at Postgres precision (microseconds, UTC).
var baseTime = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func at(minutes int) time.Time { return baseTime.Add(time.Duration(minutes) * time.Minute) }

func timePtr(t time.Time) *time.Time { return &t }

func countRows(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, testDB.QueryRow(query, args...).Scan(&n))
	return n
}
