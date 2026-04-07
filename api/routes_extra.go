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
	mux.HandleFunc("POST /v1/zone/export", s.handleZoneExport)
	mux.HandleFunc("POST /v1/zone/import", s.handleZoneImport)
	_ = http.StatusOK // keep net/http import stable if future routes need it
}
