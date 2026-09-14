package server

import (
	"context"
	"net/http"
	"time"

	"github.com/SamuelSupe/contextGate/internal/engine"
	"github.com/SamuelSupe/contextGate/internal/model"
)

func (s *Server) trialAllSemantics(w http.ResponseWriter, r *http.Request) {
	var in semanticAction
	if err := decode(r, &in); err != nil {
		semanticFailure(w, model.Fail("invalid_input", "Invalid trial request"))
		return
	}
	_, st, err := s.semanticState(r.PathValue("id"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	if st.Revision != in.Revision {
		semanticFailure(w, model.Fail("conflict", "Draft changed; reload before running trials"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	results := []map[string]any{}
	remaining := 0
	for _, en := range st.Draft.Entries {
		if en.Template == nil || !en.Template.Enabled {
			continue
		}
		if ctx.Err() != nil {
			remaining++
			continue
		}
		v, err := s.Engine.TrialTemplate(ctx, r.PathValue("id"), en.ID, in.Revision)
		item := map[string]any{"id": en.ID, "validation": v}
		if err != nil {
			item["error"] = engine.PublicError(err)
		}
		results = append(results, item)
		if model.ErrorCode(err) == "conflict" {
			semanticFailure(w, err)
			return
		}
	}
	write(w, 200, map[string]any{"results": results, "remaining": remaining})
}
