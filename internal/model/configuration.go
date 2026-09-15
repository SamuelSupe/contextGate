package model

import (
	"context"
	"strconv"
	"time"
)

type ConfigurationAgent struct {
	AdministratorID string     `json:"administrator_id,omitempty"`
	Revision        int64      `json:"revision,string"`
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	CreatedAt       time.Time  `json:"created_at"`
	ExpiresAt       time.Time  `json:"expires_at"`
	RevokedAt       *time.Time `json:"revoked_at,omitempty"`
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

func PreviewPrincipal(ctx context.Context, agentID string) Principal {
	p := AdministratorPrincipal(ctx)
	p.Admin, p.AgentID, p.Preview = agentID == "", agentID, true
	return p
}

// Cursor identities separate the acting administrator from the Agent whose
// data-source grants are being previewed, and bind token rotations.
func (p Principal) CursorIdentity() string {
	identity := p.AgentID
	if p.Admin && identity == "" {
		identity = "admin"
	}
	if p.AdministratorID != "" {
		identity += ":" + p.AdministratorID + ":" + p.ConfigurationAgentID + ":" + strconv.FormatInt(p.CredentialVersion, 10) + ":" + p.SessionIdentity
	}
	return identity
}

func (p Principal) AttributeAudit(a Audit) Audit {
	a.AdministratorID, a.AdministratorUsername = p.AdministratorID, p.AdministratorUsername
	a.ConfigurationAgentID, a.Channel = p.ConfigurationAgentID, p.Channel
	if p.ConfigurationAgentID != "" {
		a.ActorType = "configuration_agent"
	} else if p.AdministratorID != "" {
		a.ActorType = "administrator"
	} else if p.SystemCheck {
		a.ActorType = "system"
	} else if p.AgentID != "" {
		a.ActorType = "query_agent"
	}
	return a
}
