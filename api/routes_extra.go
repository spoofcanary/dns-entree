package api

import "net/http"

// registerExtraRoutes wires the plan 06-03 handlers onto the Server mux: DC
// discover + apply-url, 4 template endpoints, migrate, zone export, zone
// import. Called from NewServer after wave-1 routes are registered.
func registerExtraRoutes(s *Server) {
	mux := s.mux
	mux.HandleFunc("POST /v1/dc/discover", s.handleDCDiscover)
	mux.HandleFunc("POST /v1/dc/apply-url", s.handleDCApplyURL)

	mux.HandleFunc("POST /v1/templates/sync", s.handleTemplatesSync)
	mux.HandleFunc("GET /v1/templates", s.handleTemplatesList)
	mux.HandleFunc("GET /v1/templates/{provider}/{service}", s.handleTemplateGet)
	mux.HandleFunc("POST /v1/templates/{provider}/{service}/resolve", s.handleTemplateResolve)

	mux.HandleFunc("POST /v1/migrate", s.handleMigrate)
	// Stateful migration routes (Phase 07-05). Handlers parse the {id}
	// segment via extractPathID(r.URL.Path) rather than PathValue, so the
	// stdlib mux pattern here just needs to match the shape.
	mux.HandleFunc("POST /v1/migrate/preview", s.handleMigratePreview)
	mux.HandleFunc("POST /v1/migrate/{id}/apply", s.handleMigrateApply)
	mux.HandleFunc("POST /v1/migrate/{id}/verify", s.handleMigrateVerify)
	mux.HandleFunc("GET /v1/migrate/{id}", s.handleMigrateGet)
	mux.HandleFunc("GET /v1/migrate", s.handleMigrateList)
	mux.HandleFunc("DELETE /v1/migrate/{id}", s.handleMigrateDelete)
	mux.HandleFunc("POST /v1/zone/export", s.handleZoneExport)
	mux.HandleFunc("POST /v1/zone/import", s.handleZoneImport)
	_ = http.StatusOK // keep net/http import stable if future routes need it
}
