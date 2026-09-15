package store

import (
	"database/sql"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"time"
)

func (s *Store) CreateAdministratorSession(id, token, csrf string, expires time.Time, hash string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// This row lock serializes issuance with password replacement and account
	// revocation; a password verified before either cannot recreate a session.
	a, err := scanAdministrator(tx.QueryRow("SELECT "+administratorColumns+" FROM administrators WHERE id=$1 FOR UPDATE", id))
	if err == sql.ErrNoRows || err == nil && (!a.Active() || !secure.Equal(a.PasswordHash, hash)) {
		return model.Fail("unauthorized", "Account changed; sign in again")
	}
	if err != nil {
		return err
	}
	if a.MustChangePassword && a.TemporaryExpiresAt.Before(expires) {
		expires = *a.TemporaryExpiresAt
	}
	if _, err = tx.Exec("INSERT INTO sessions(hash,csrf,expires,administrator_id,security_version) VALUES($1,$2,$3,$4,$5)", secure.Hash(token), csrf, expires.Unix(), id, a.SecurityVersion); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE administrators SET last_login_at=$1 WHERE id=$2", time.Now().UTC(), id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AdministratorSession(token string) (model.Administrator, string, error) {
	var id, csrf string
	var version int64
	err := s.DB.QueryRow("SELECT administrator_id,csrf,security_version FROM sessions WHERE hash=$1 AND expires>$2", secure.Hash(token), time.Now().Unix()).Scan(&id, &csrf, &version)
	if err != nil {
		return model.Administrator{}, "", err
	}
	a, err := s.Administrator(id)
	if err == nil && (!a.Active() || a.SecurityVersion != version) {
		err = model.Fail("unauthorized", "Account changed; sign in again")
	}
	return a, csrf, err
}

func (s *Store) SoleAdministrator(username string) (model.Administrator, error) {
	if username != "" {
		return s.AdministratorByUsername(username)
	}
	all, err := s.Administrators()
	if err != nil {
		return model.Administrator{}, err
	}
	if len(all) == 0 {
		return model.Administrator{}, model.Fail("invalid_input", "Initialize an administrator account first")
	}
	if len(all) != 1 {
		return model.Administrator{}, model.Fail("invalid_input", "Specify --username when more than one administrator exists")
	}
	return all[0], nil
}
