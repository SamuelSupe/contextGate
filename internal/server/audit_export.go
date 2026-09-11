package server

import (
	"net/http"

	"github.com/SamuelSupe/mcpdbhub/internal/auditexport"
	"github.com/SamuelSupe/mcpdbhub/internal/model"
)

func (s *Server) auditExportSettings(w http.ResponseWriter, r *http.Request) {
	view, err := s.AuditExport.View(r.Context())
	if err != nil {
		fail(w, 500, err)
		return
	}
	write(w, 200, view)
}

func (s *Server) saveAuditExport(w http.ResponseWriter, r *http.Request) {
	var update auditexport.Update
	if err := decode(r, &update); err != nil {
		fail(w, 400, model.Fail("invalid_input", "Invalid audit export settings."))
		return
	}
	if err := s.AuditExport.Update(r.Context(), update); err != nil {
		code := http.StatusInternalServerError
		switch model.ErrorCode(err) {
		case "invalid_input":
			code = 400
		case "conflict":
			code = 409
		}
		fail(w, code, err)
		return
	}
	s.auditExportSettings(w, r)
}

func (s *Server) testAuditExport(w http.ResponseWriter, r *http.Request) {
	var update auditexport.Update
	if err := decode(r, &update); err != nil {
		fail(w, 400, model.Fail("invalid_input", "Invalid audit export settings."))
		return
	}
	result, err := s.AuditExport.Test(r.Context(), update)
	if err != nil {
		code := 400
		if model.ErrorCode(err) == "conflict" {
			code = 409
		}
		fail(w, code, err)
		return
	}
	write(w, 200, result)
}
