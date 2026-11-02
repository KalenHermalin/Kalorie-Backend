package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

type remindMeStore struct {
	db *sql.DB
}
type RemindMeRepo interface {
	Remind_user(ctx context.Context, email string) error
}

func NewRemindMeStore(db *sql.DB) *remindMeStore {
	return &remindMeStore{
		db: db,
	}

}

var ErrEmailExists = errors.New("email already exists")

func (us *remindMeStore) Remind_user(ctx context.Context, email string) error {
	query := `INSERT INTO remind_user (email) VALUES ($1)`
	_, err := us.db.ExecContext(ctx, query, email)
	var pqError *pq.Error
	if errors.As(err, &pqError) {
		if pqError.Code.Name() == "unique_violation" {
			return fmt.Errorf("remind user: %w", ErrEmailExists)
		}

	}
	if err != nil {
		return fmt.Errorf("remind user: %w", err)
	}
	return nil

}
