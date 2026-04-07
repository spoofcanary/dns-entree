package api

// registerCoreRoutes attaches the five CLI-mirror business endpoints to the
// server mux. Called from NewServer after wave-1 scaffolding.
func registerCoreRoutes(s *Server) {
	s.mux.HandleFunc("POST /v1/detect", s.handleDetect)
	s.mux.HandleFunc("POST /v1/verify", s.handleVerify)
	s.mux.HandleFunc("POST /v1/spf-merge", s.handleSPFMerge)
	s.mux.HandleFunc("POST /v1/apply", s.handleApply)
	s.mux.HandleFunc("POST /v1/apply/template", s.handleApplyTemplate)
}
