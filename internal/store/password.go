package store

import (
	"errors"
)

// ReplaceAdminPassword keeps credentials and grants intact and invalidates every
// administrator session in the same transaction as the password change.
func (s *Store) ReplaceAdminPassword(hash string) error {
	s.Mutations.Lock()
	defer s.Mutations.Unlock()
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec("UPDATE kv SET value=? WHERE key='admin_password'", hash)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("administrator is not initialized; use first-time setup")
	}
	if _, err = tx.Exec("DELETE FROM sessions"); err != nil {
		return err
	}
	return tx.Commit()
}
