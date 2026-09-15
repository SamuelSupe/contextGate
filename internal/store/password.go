package store

import (
	"context"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"time"
)

func (s *Store) ReplaceAdminPassword(hash string) error {
	return s.ResetAdministratorPasswordCLI("", hash)
}

func (s *Store) ResetAdministratorPasswordCLI(username, hash string) error {
	a, err := s.SoleAdministrator(username)
	if err != nil {
		return err
	}
	audit := model.Audit{At: time.Now().UTC(), EventKind: "security", Operation: "administrator.recover_password", ActorType: "local_operator", Channel: "cli", ResourceID: a.ID, RequestID: secure.Random(16), ChangedFields: "credentials", ErrorCode: "operation_pending"}
	if err = s.Audit(audit); err != nil {
		return model.Fail("audit_unavailable", "Password was not changed because the audit log is unavailable")
	}
	_, err = s.replaceAdministratorPassword(context.Background(), a.ID, "", hash, "", false, true, 0)
	audit.ErrorCode = ""
	if err != nil {
		audit.ErrorCode = model.ErrorCode(err)
	}
	audit.ElapsedMS = time.Since(audit.At).Milliseconds()
	if outcomeErr := s.Audit(audit); outcomeErr != nil {
		return model.Fail("audit_unavailable", "Recovery outcome could not be recorded; check the pending audit event before retrying")
	}
	return err
}

func (s *Store) ChangeAdministratorPassword(ctx context.Context, id, expected, hash, keepSession string) (model.Administrator, error) {
	return s.replaceAdministratorPassword(ctx, id, expected, hash, keepSession, false, false, 0)
}
func (s *Store) ResetAdministratorPassword(ctx context.Context, id, hash string, revision int64) (model.Administrator, error) {
	return s.replaceAdministratorPassword(ctx, id, "", hash, "", true, true, revision)
}

func (s *Store) replaceAdministratorPassword(ctx context.Context, id, expected, hash, keepSession string, temporary, revoke bool, revision int64) (model.Administrator, error) {
	s.Mutations.Lock()
	defer s.Mutations.Unlock()
	if err := model.CheckConfigurationContext(ctx); err != nil {
		return model.Administrator{}, err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return model.Administrator{}, err
	}
	defer tx.Rollback()
	a, err := scanAdministrator(tx.QueryRow("SELECT "+administratorColumns+" FROM administrators WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		return a, err
	}
	if expected != "" && a.PasswordHash != expected || revision > 0 && a.Revision != revision {
		return a, model.Fail("conflict", "Administrator changed; reload or sign in again")
	}
	a.PasswordHash, a.MustChangePassword, a.Revision = hash, temporary, a.Revision+1
	a.TemporaryExpiresAt = nil
	if temporary {
		expiry := time.Now().UTC().Add(24 * time.Hour)
		a.TemporaryExpiresAt = &expiry
	}
	if revoke {
		a.SecurityVersion++
		if err = revokeAdministratorAccess(tx, id); err != nil {
			return a, err
		}
	} else {
		if _, err = tx.Exec("DELETE FROM sessions WHERE administrator_id=$1 AND hash<>$2", id, keepSession); err != nil {
			return a, err
		}
	}
	_, err = tx.Exec("UPDATE administrators SET password_hash=$1,must_change_password=$2,temporary_expires_at=$3,revision=$4,security_version=$5 WHERE id=$6", hash, temporary, a.TemporaryExpiresAt, a.Revision, a.SecurityVersion, id)
	if err != nil {
		return a, err
	}
	return a, tx.Commit()
}
