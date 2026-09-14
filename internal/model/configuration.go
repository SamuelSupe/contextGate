package model

import (
	"context"
	"time"
)

type ConfigurationAgent struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

type configurationPrincipalKey struct{}

// WithConfigurationPrincipal carries a server-authenticated configuration caller
// through administrator workflows, including their nested query audit records.
func WithConfigurationPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, configurationPrincipalKey{}, p)
}

func AdministratorPrincipal(ctx context.Context) Principal {
	if p, ok := ctx.Value(configurationPrincipalKey{}).(Principal); ok {
		return p
	}
	return Principal{Admin: true, Preview: true}
}

func CheckConfigurationContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p := AdministratorPrincipal(ctx)
	if p.CredentialValid != nil && !p.CredentialValid() {
		return Fail("unauthorized", "Configuration credential revoked or expired")
	}
	return nil
}
