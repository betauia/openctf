package health

import "net/http"

func RegisterHealthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", healthHandler)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
