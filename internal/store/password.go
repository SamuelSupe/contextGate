package store

import (
	"errors"

	"github.com/SamuelSupe/contextGate/internal/model"
)

// ReplaceAdminPassword keeps credentials and grants intact and invalidates every
// administrator session in the same transaction as the password change.
func (s *Store) ReplaceAdminPassword(hash string) error {
	return s.replaceAdminPassword("", hash)
}

// ChangeAdminPassword rejects a password verification made obsolete by another
// change, before either the password or the current sessions can be modified.
func (s *Store) ChangeAdminPassword(expected, hash string) error {
	if expected == "" {
		return model.Fail("conflict", "Administrator password changed; sign in again")
	}
	return s.replaceAdminPassword(expected, hash)
}

func (s *Store) replaceAdminPassword(expected, hash string) error {
	s.Mutations.Lock()
	defer s.Mutations.Unlock()
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec("UPDATE kv SET value=$1 WHERE key='admin_password' AND ($2='' OR value=$3)", hash, expected, expected)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		if expected != "" {
			return model.Fail("conflict", "Administrator password changed; sign in again")
		}
		return errors.New("administrator is not initialized; use first-time setup")
	}
	if _, err = tx.Exec("DELETE FROM sessions"); err != nil {
		return err
	}
	return tx.Commit()
}
