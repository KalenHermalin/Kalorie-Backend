package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// remind_user has no link to users, so clearTables doesn't empty it; these tests clean up their own rows.
func resetRemindUsers(t *testing.T) {
	t.Helper()
	clear := func() {
		_, err := testDB.Exec(`DELETE FROM remind_user WHERE email LIKE '%@remind.test'`)
		require.NoError(t, err)
	}
	clear()
	t.Cleanup(clear)
}

func TestRemindUserNewEmail(t *testing.T) {
	resetRemindUsers(t)
	remindStore := NewRemindMeStore(testDB)

	require.NoError(t, remindStore.Remind_user(context.Background(), "new@remind.test"))
	assert.Equal(t, 1, countRows(t, `SELECT count(*) FROM remind_user WHERE email = 'new@remind.test'`))
}

func TestRemindUserDuplicateEmail(t *testing.T) {
	resetRemindUsers(t)
	remindStore := NewRemindMeStore(testDB)
	require.NoError(t, remindStore.Remind_user(context.Background(), "dupe@remind.test"))

	err := remindStore.Remind_user(context.Background(), "dupe@remind.test")
	assert.ErrorIs(t, err, ErrEmailExists)
	assert.Equal(t, 1, countRows(t, `SELECT count(*) FROM remind_user WHERE email = 'dupe@remind.test'`))
}

func TestRemindUserDifferentEmails(t *testing.T) {
	resetRemindUsers(t)
	remindStore := NewRemindMeStore(testDB)

	require.NoError(t, remindStore.Remind_user(context.Background(), "one@remind.test"))
	require.NoError(t, remindStore.Remind_user(context.Background(), "two@remind.test"))
	assert.Equal(t, 2, countRows(t, `SELECT count(*) FROM remind_user WHERE email LIKE '%@remind.test'`))
}

func TestRemindUserCancelledContext(t *testing.T) {
	resetRemindUsers(t)
	remindStore := NewRemindMeStore(testDB)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := remindStore.Remind_user(ctx, "cancelled@remind.test")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrEmailExists)
}
