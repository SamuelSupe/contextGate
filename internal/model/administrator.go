package model

import "time"

const (
	RoleSuperAdministrator = "super_admin"
	RoleAdministrator      = "admin"
)

type Administrator struct {
	ID                 string     `json:"id"`
	Username           string     `json:"username"`
	DisplayName        string     `json:"display_name"`
	Role               string     `json:"role"`
	Enabled            bool       `json:"enabled"`
	MustChangePassword bool       `json:"must_change_password"`
	TemporaryExpiresAt *time.Time `json:"temporary_expires_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	LastLoginAt        *time.Time `json:"last_login_at,omitempty"`
	Revision           int64      `json:"revision,string"`
	SecurityVersion    int64      `json:"-"`
	PasswordHash       string     `json:"-"`
}

func (a Administrator) Active() bool {
	return a.ID != "" && a.Enabled && (!a.MustChangePassword || a.TemporaryExpiresAt != nil && a.TemporaryExpiresAt.After(time.Now()))
}
