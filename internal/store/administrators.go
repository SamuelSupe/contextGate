package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
)

const administratorColumns = "id,username,display_name,role,enabled,password_hash,must_change_password,temporary_expires_at,created_at,last_login_at,revision,security_version"

func scanAdministrator(row interface{ Scan(...any) error }) (model.Administrator, error) {
	var a model.Administrator
	err := row.Scan(&a.ID, &a.Username, &a.DisplayName, &a.Role, &a.Enabled, &a.PasswordHash, &a.MustChangePassword, &a.TemporaryExpiresAt, &a.CreatedAt, &a.LastLoginAt, &a.Revision, &a.SecurityVersion)
	return a, err
}

var administratorUsername = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)

func NormalizeAdministrator(username, name, role string) (string, string, error) {
	username, name = strings.ToLower(strings.TrimSpace(username)), strings.TrimSpace(name)
	if !administratorUsername.MatchString(username) || name == "" || len(name) > 120 || role != model.RoleAdministrator && role != model.RoleSuperAdministrator {
		return "", "", model.Fail("invalid_input", "Use a 3-64 character username (letters, numbers, dot, underscore or hyphen), a display name and a valid role")
	}
	return username, name, nil
}

func (s *Store) Administrator(id string) (model.Administrator, error) {
	return scanAdministrator(s.DB.QueryRow("SELECT "+administratorColumns+" FROM administrators WHERE id=$1", id))
}
func (s *Store) AdministratorByUsername(username string) (model.Administrator, error) {
	return scanAdministrator(s.DB.QueryRow("SELECT "+administratorColumns+" FROM administrators WHERE username=$1", strings.ToLower(strings.TrimSpace(username))))
}
func (s *Store) Administrators() ([]model.Administrator, error) {
	rows, err := s.DB.Query("SELECT " + administratorColumns + " FROM administrators ORDER BY username")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Administrator{}
	for rows.Next() {
		a, err := scanAdministrator(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Store) AdministratorInitialized() (bool, error) {
	var yes bool
	err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM administrators)").Scan(&yes)
	return yes, err
}

func insertAdministrator(tx *sql.Tx, a model.Administrator) error {
	_, err := tx.Exec("INSERT INTO administrators("+administratorColumns+") VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)", a.ID, a.Username, a.DisplayName, a.Role, a.Enabled, a.PasswordHash, a.MustChangePassword, a.TemporaryExpiresAt, a.CreatedAt, a.LastLoginAt, a.Revision, a.SecurityVersion)
	if err != nil {
		return err
	}
	identity := model.ConfigurationAgent{ID: "cfg_" + secure.Random(16), Name: a.DisplayName, AdministratorID: a.ID, CreatedAt: a.CreatedAt, Revision: 1}
	b, err := json.Marshal(identity)
	if err != nil {
		return err
	}
	_, err = tx.Exec("INSERT INTO configuration_agents(id,value,administrator_id) VALUES($1,$2,$3)", identity.ID, string(b), a.ID)
	return err
}

// The legacy identity cannot be attributed to a person. Retain its history but
// invalidate its credentials and sessions in the same transaction as migration.
func migrateAdministrators(tx *sql.Tx) error {
	var done bool
	if err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM kv WHERE key='administrator_accounts_v1')").Scan(&done); err != nil {
		return err
	}
	if done {
		return nil
	}
	var hash string
	err := tx.QueryRow("SELECT value FROM kv WHERE key='admin_password'").Scan(&hash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		a := model.Administrator{ID: "adm_" + secure.Random(16), Username: "admin", DisplayName: "Administrator", Role: model.RoleSuperAdministrator, Enabled: true, PasswordHash: hash, CreatedAt: time.Now().UTC(), Revision: 1, SecurityVersion: 1}
		if err = insertAdministrator(tx, a); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("DELETE FROM sessions"); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE configuration_agents SET token_hash=NULL,value=(value::jsonb || jsonb_build_object('revoked_at',CURRENT_TIMESTAMP))::text WHERE administrator_id IS NULL`); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM kv WHERE key='admin_password'"); err != nil {
		return err
	}
	_, err = tx.Exec("INSERT INTO kv(key,value) VALUES('administrator_accounts_v1','1')")
	return err
}

func (s *Store) SetupAdministrator(username, name, hash string) (model.Administrator, error) {
	username, name, err := NormalizeAdministrator(username, name, model.RoleSuperAdministrator)
	if err != nil {
		return model.Administrator{}, err
	}
	a := model.Administrator{ID: "adm_" + secure.Random(16), Username: username, DisplayName: name, Role: model.RoleSuperAdministrator, Enabled: true, PasswordHash: hash, CreatedAt: time.Now().UTC(), Revision: 1, SecurityVersion: 1}
	tx, err := s.DB.Begin()
	if err != nil {
		return a, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("LOCK TABLE administrators IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		return a, err
	}
	var count int
	if err = tx.QueryRow("SELECT count(*) FROM administrators").Scan(&count); err != nil {
		return a, err
	}
	if count != 0 {
		return a, model.Fail("already_initialized", "Administrator is already initialized")
	}
	if err = insertAdministrator(tx, a); err != nil {
		return a, err
	}
	if _, err = tx.Exec("DELETE FROM kv WHERE key='setup_hash'"); err != nil {
		return a, err
	}
	return a, tx.Commit()
}

func (s *Store) CreateAdministrator(ctx context.Context, username, name, role, hash string) (model.Administrator, error) {
	username, name, err := NormalizeAdministrator(username, name, role)
	if err != nil {
		return model.Administrator{}, err
	}
	expires := time.Now().UTC().Add(24 * time.Hour)
	a := model.Administrator{ID: "adm_" + secure.Random(16), Username: username, DisplayName: name, Role: role, Enabled: true, PasswordHash: hash, MustChangePassword: true, TemporaryExpiresAt: &expires, CreatedAt: time.Now().UTC(), Revision: 1, SecurityVersion: 1}
	s.Mutations.Lock()
	defer s.Mutations.Unlock()
	if err = model.CheckConfigurationContext(ctx); err != nil {
		return a, err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return a, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("LOCK TABLE administrators IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		return a, err
	}
	var exists bool
	if err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM administrators WHERE username=$1)", username).Scan(&exists); err != nil {
		return a, err
	}
	if exists {
		return a, model.Fail("conflict", "Username is already in use")
	}
	if err = insertAdministrator(tx, a); err != nil {
		return a, err
	}
	return a, tx.Commit()
}

func revokeAdministratorAccess(tx *sql.Tx, id string) error {
	if _, err := tx.Exec("DELETE FROM sessions WHERE administrator_id=$1", id); err != nil {
		return err
	}
	_, err := tx.Exec(`UPDATE configuration_agents SET token_hash=NULL,value=(value::jsonb || jsonb_build_object('revoked_at',CURRENT_TIMESTAMP,'revision',((value::jsonb->>'revision')::bigint+1)::text))::text WHERE administrator_id=$1`, id)
	return err
}

func (s *Store) UpdateAdministrator(ctx context.Context, id, name, role string, enabled bool, revision int64) (model.Administrator, bool, error) {
	s.Mutations.Lock()
	defer s.Mutations.Unlock()
	if err := model.CheckConfigurationContext(ctx); err != nil {
		return model.Administrator{}, false, err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return model.Administrator{}, false, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("LOCK TABLE administrators IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		return model.Administrator{}, false, err
	}
	a, err := scanAdministrator(tx.QueryRow("SELECT "+administratorColumns+" FROM administrators WHERE id=$1", id))
	if err != nil {
		return a, false, err
	}
	if a.Revision != revision {
		return a, false, model.Fail("conflict", "Administrator changed; reload before saving")
	}
	_, name, err = NormalizeAdministrator(a.Username, name, role)
	if err != nil {
		return a, false, err
	}
	changed := a.Role != role || a.Enabled != enabled
	if id == model.AdministratorPrincipal(ctx).AdministratorID && (!enabled || a.Role != role) {
		return a, false, model.Fail("forbidden", "You cannot disable yourself or change your own role")
	}
	if a.Role == model.RoleSuperAdministrator && a.Enabled && (role != a.Role || !enabled) {
		var count int
		if err = tx.QueryRow("SELECT count(*) FROM administrators WHERE enabled AND role='super_admin'").Scan(&count); err != nil {
			return a, false, err
		}
		if count <= 1 {
			return a, false, model.Fail("conflict", "At least one enabled super administrator is required")
		}
	}
	a.DisplayName, a.Role, a.Enabled, a.Revision = name, role, enabled, a.Revision+1
	if changed {
		a.SecurityVersion++
		if err = revokeAdministratorAccess(tx, id); err != nil {
			return a, false, err
		}
	}
	_, err = tx.Exec("UPDATE administrators SET display_name=$1,role=$2,enabled=$3,revision=$4,security_version=$5 WHERE id=$6", a.DisplayName, a.Role, a.Enabled, a.Revision, a.SecurityVersion, id)
	if err != nil {
		return a, false, err
	}
	return a, changed, tx.Commit()
}
