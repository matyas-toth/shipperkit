package store

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Store) ACMEContact(ctx context.Context) (string, error) {
	var contact string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='acme_contact'").Scan(&contact)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return contact, err
}
