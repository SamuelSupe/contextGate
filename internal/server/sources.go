package server

import (
	"github.com/SamuelSupe/mcpdbhub/internal/adapter"
	"github.com/SamuelSupe/mcpdbhub/internal/engine"
	"github.com/SamuelSupe/mcpdbhub/internal/model"
	"github.com/SamuelSupe/mcpdbhub/internal/secure"
	"net/http"
	"time"
)

type sourceView struct {
	model.Source
	Capability adapter.SourceCapability `json:"capability"`
}

func publicSource(src model.Source) sourceView {
	return sourceView{src.Public(), adapter.ForSource(src)}
}

func (s *Server) sources(w http.ResponseWriter, r *http.Request) {
	sources, e := s.Store.Sources()
	if e != nil {
		fail(w, 500, e)
		return
	}
	out := []sourceView{}
	for _, src := range sources {
		out = append(out, publicSource(src))
	}
	write(w, 200, out)
}
func (s *Server) saveSource(w http.ResponseWriter, r *http.Request) {
	var input sourceInput
	if e := decode(r, &input); e != nil {
		fail(w, 400, model.Fail("invalid_input", e.Error()))
		return
	}
	src := input.Source
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	id := r.PathValue("id")
	var old model.Source
	if id != "" {
		var e error
		old, e = s.Store.Source(id)
		if e != nil {
			fail(w, 404, model.Fail("not_found", "data source not found"))
			return
		}
		if src.Revision != old.Revision {
			fail(w, 409, model.Fail("conflict", "data source changed; reload before saving"))
			return
		}
		src.ID = id
		if e := input.credentials(&old); e != nil {
			fail(w, 400, e)
			return
		}
		src = input.Source
		src.ID = id
	} else {
		if e := input.credentials(nil); e != nil {
			fail(w, 400, e)
			return
		}
		src = input.Source
		src.ID = "src_" + secure.Random(16)
	}
	if src.QueryAccessMode == "" {
		src.QueryAccessMode = old.QueryMode()
	}
	if src.QueryAccessMode != "native_and_templates" && src.QueryAccessMode != "templates_only" {
		fail(w, 400, model.Fail("invalid_input", "Invalid query access mode"))
		return
	}
	src.ObservedVersion = old.ObservedVersion
	src.ConnectionRevision = old.ConnectionRevision
	if id == "" || !src.SameConnection(old) {
		src.ConnectionRevision++
		src.ObservedVersion = ""
	}
	if e := adapter.ValidateSource(&src, s.FileRoot); e != nil {
		fail(w, 400, model.Fail("invalid_input", e.Error()))
		return
	}
	queryChanged := id == "" || !src.SameQueryConfig(old)
	src.Revision = old.Revision + 1
	src.QueryRevision = src.Revision
	src.Probe = nil
	if !queryChanged {
		src.QueryRevision = old.ExecutionRevision()
		src.Probe = old.Probe
	}
	src.HasSecret = false
	if e := s.Store.SaveSource(src); e != nil {
		fail(w, 500, e)
		return
	}
	if queryChanged {
		s.Engine.InvalidateSource(src.ID)
	}
	write(w, 200, publicSource(src))
}
func (s *Server) deleteSource(w http.ResponseWriter, r *http.Request) {
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	id := r.PathValue("id")
	if e := s.Store.DeleteSource(id); e != nil {
		fail(w, 500, e)
		return
	}
	s.Engine.InvalidateSource(id)
	write(w, 200, map[string]any{"ok": true})
}
func (s *Server) testSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var src model.Source
	var e error
	if id != "" {
		src, e = s.Store.Source(id)
	} else {
		var input sourceInput
		e = decode(r, &input)
		if e == nil {
			var old *model.Source
			if input.ID != "" {
				value, err := s.Store.Source(input.ID)
				if err != nil {
					e = err
				} else {
					old = &value
				}
			}
			if e == nil {
				e = input.credentials(old)
				src = input.Source
			}
		}
	}
	if e != nil {
		fail(w, 400, model.Fail("invalid_input", "data source not found or invalid configuration"))
		return
	}
	if e = adapter.ValidateSource(&src, s.FileRoot); e != nil {
		fail(w, 400, model.Fail("invalid_input", e.Error()))
		return
	}
	started := time.Now()
	requestID := secure.Random(16)
	probe, probeErr := s.Engine.Probe(r.Context(), src)
	probe.CheckedAt = time.Now()
	if probeErr != nil {
		probe = model.Probe{Connected: false, PermissionStatus: "unverified", CheckedAt: time.Now(), Evidence: []string{}, Error: engine.PublicError(probeErr)}
		probe.Error.RequestID = requestID
	}
	record := model.Audit{RequestID: requestID, At: started, AgentID: "admin", SourceID: src.ID, Operation: "test_connection", ElapsedMS: time.Since(started).Milliseconds(), Preview: true}
	if probe.Error != nil {
		record.ErrorCode = probe.Error.Code
		record.NativeCode = probe.Error.NativeCode
	}
	if err := s.Store.Audit(record); err != nil {
		fail(w, 500, model.Fail("audit_unavailable", "Connection check could not be recorded."))
		return
	}
	if id != "" {
		s.Store.Mutations.Lock()
		defer s.Store.Mutations.Unlock()
		fresh, e := s.Store.Source(id)
		if e != nil || fresh.Revision != src.Revision {
			fail(w, 409, model.Fail("conflict", "data source changed during verification"))
			return
		}
		if probe.Connected && probe.ServerVersion != "" && fresh.ObservedVersion != probe.ServerVersion {
			known := fresh.ObservedVersion != ""
			fresh.ObservedVersion = probe.ServerVersion
			if known {
				fresh.ConnectionRevision++
				fresh.Revision++
				fresh.QueryRevision = fresh.Revision
				s.Engine.InvalidateSource(id)
			}
		}
		fresh.Probe = &probe
		if e = s.Store.SaveSource(fresh); e != nil {
			fail(w, 500, e)
			return
		}
	}
	if probeErr != nil {
		fail(w, 400, probe.Error)
		return
	}
	write(w, 200, probe)
}
func (s *Server) query(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Operation string      `json:"operation"`
		SourceID  string      `json:"source_id"`
		AgentID   string      `json:"agent_id"`
		Cursor    *string     `json:"cursor"`
		Query     model.Query `json:"query"`
	}
	if e := decode(r, &in); e != nil {
		fail(w, 400, model.Fail("invalid_input", e.Error()))
		return
	}
	if in.SourceID != "" {
		in.Query.SourceID = in.SourceID
	}
	if in.Cursor != nil {
		in.Query.Cursor = *in.Cursor
	}
	p := model.Principal{Admin: in.AgentID == "", AgentID: in.AgentID, Preview: true}
	res, e := s.Engine.Execute(r.Context(), p, in.Operation, in.Query)
	if e != nil {
		fail(w, 400, e)
		return
	}
	write(w, 200, res)
}
func (s *Server) discover(w http.ResponseWriter, r *http.Request) {
	op := r.URL.Query().Get("operation")
	if op == "" {
		op = "objects"
	}
	if op != "objects" && op != "namespaces" && op != "describe" {
		fail(w, 400, model.Fail("invalid_input", "invalid discovery operation"))
		return
	}
	res, e := s.Engine.Execute(r.Context(), model.Principal{Admin: r.URL.Query().Get("agent_id") == "", AgentID: r.URL.Query().Get("agent_id"), Preview: true}, op, model.Query{SourceID: r.PathValue("id"), Namespace: r.URL.Query().Get("namespace"), Object: r.URL.Query().Get("object"), Cursor: r.URL.Query().Get("cursor")})
	if e != nil {
		fail(w, 400, e)
		return
	}
	write(w, 200, res)
}
