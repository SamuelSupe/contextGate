package server

import (
	"encoding/json"
	"github.com/SamuelSupe/contextGate/internal/model"
)

type sourceInput struct {
	model.Source
	Capability    json.RawMessage `json:"capability,omitempty"`
	ClearPassword bool            `json:"clear_password"`
	ClearToken    bool            `json:"clear_token"`
}

func (in *sourceInput) credentials(old *model.Source) error {
	if in.ClearPassword && in.Password != "" || in.ClearToken && in.Token != "" {
		return model.Fail("invalid_input", "Choose either replace or clear for each credential.")
	}
	if old != nil && old.Kind == in.Kind {
		if in.Password == "" && !in.ClearPassword {
			in.Password = old.Password
		}
		if in.Token == "" && !in.ClearToken {
			in.Token = old.Token
		}
	}
	switch in.AuthMode {
	case "": // Existing API clients retain their credential update behavior.
	case "none":
		in.Password = ""
		in.Token = ""
		in.Username = ""
	case "password":
		in.Token = ""
	case "token":
		if in.Kind != "influxdb" && in.Kind != "elasticsearch" && in.Kind != "opensearch" {
			return model.Fail("invalid_input", "This database does not support token authentication.")
		}
		in.Username = ""
		in.Password = ""
	default:
		return model.Fail("invalid_input", "Invalid authentication method.")
	}
	return nil
}
