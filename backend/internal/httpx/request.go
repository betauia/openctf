package httpx

import (
	"encoding/json/v2"
	"net/http"
	"uuid"
)

// Decode reads a JSON body (capped at 1 MiB) into v. On failure it writes a 422 and returns false.
func Decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.UnmarshalRead(http.MaxBytesReader(w, r.Body, 1<<20), v); err != nil {
		Error(w, http.StatusUnprocessableEntity, "Invalid request body")
		return false
	}
	return true
}

// PathID parses the UUID path value name (e.g. {id} in "GET /api/teams/{id}").
// On failure it writes a 422 and returns false.
func PathID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "Invalid "+name)
		return uuid.UUID{}, false
	}
	return id, true
}
