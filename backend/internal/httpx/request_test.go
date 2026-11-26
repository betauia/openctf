package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"
)

type payload struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func decode(t *testing.T, body string) (payload, bool, *httptest.ResponseRecorder) {
	t.Helper()
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	var p payload
	ok := Decode(rec, r, &p)
	return p, ok, rec
}

func TestDecodeReadsAJSONBody(t *testing.T) {
	p, ok, rec := decode(t, `{"name":"abc","count":3}`)

	if !ok || p != (payload{"abc", 3}) {
		t.Errorf("got %+v ok=%v, want {abc 3} true", p, ok)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("a successful decode wrote %q", rec.Body)
	}
}

func TestDecodeIgnoresUnknownFields(t *testing.T) {
	p, ok, _ := decode(t, `{"name":"abc","extra":true}`)

	if !ok || p.Name != "abc" {
		t.Errorf("got %+v ok=%v, want the known field decoded", p, ok)
	}
}

func TestDecodeRejectsABadBodyWith422(t *testing.T) {
	big := `{"name":"` + strings.Repeat("a", 1<<20) + `"}` // a little over 1 MiB
	tests := map[string]string{
		"empty body":              "",
		"not JSON":                "hello",
		"truncated JSON":          `{"name":"abc"`,
		"wrong field type":        `{"name":5}`,
		"a string, not an object": `"abc"`,
		"an array":                `[1,2]`,
		"over the 1 MiB cap":      big,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			_, ok, rec := decode(t, body)

			if ok {
				t.Fatal("Decode reported success")
			}
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), `"Invalid request body"`) {
				t.Errorf("body = %s, want the Invalid request body detail", rec.Body)
			}
		})
	}
}

func TestDecodeAcceptsABodyJustUnderTheCap(t *testing.T) {
	body := `{"name":"` + strings.Repeat("a", (1<<20)-100) + `"}`

	p, ok, _ := decode(t, body)

	if !ok || len(p.Name) != (1<<20)-100 {
		t.Errorf("ok=%v, name length %d; want a body under 1 MiB to decode", ok, len(p.Name))
	}
}

// pathID runs PathID on a request routed through a mux, which is what fills in the path values.
func pathID(t *testing.T, name, value string) (uuid.UUID, bool, *httptest.ResponseRecorder) {
	t.Helper()
	var got uuid.UUID
	var ok bool
	mux := http.NewServeMux()
	mux.HandleFunc("GET /things/{"+name+"}", func(w http.ResponseWriter, r *http.Request) {
		got, ok = PathID(w, r, name)
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/things/"+value, nil))
	return got, ok, rec
}

func TestPathIDParsesAUUID(t *testing.T) {
	want := uuid.NewV7()

	got, ok, rec := pathID(t, "id", want.String())

	if !ok || got != want {
		t.Errorf("got %v ok=%v, want %v true", got, ok, want)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("a valid id wrote %q", rec.Body)
	}
}

func TestPathIDRejectsAnythingElseWith422(t *testing.T) {
	for _, value := range []string{"not-a-uuid", "123", "01a2100a-2aa2-7704-94f9", "g1a2100a-2aa2-7704-94f9-b2e5120bffdb"} {
		t.Run(value, func(t *testing.T) {
			_, ok, rec := pathID(t, "id", value)

			if ok || rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("ok=%v status=%d, want false and 422", ok, rec.Code)
			}
			if !strings.Contains(rec.Body.String(), `"Invalid id"`) {
				t.Errorf("body = %s, want the Invalid id detail", rec.Body)
			}
		})
	}
}

func TestPathIDNamesTheParameterInTheError(t *testing.T) {
	_, ok, rec := pathID(t, "fid", "nope")

	if ok || !strings.Contains(rec.Body.String(), `"Invalid fid"`) {
		t.Errorf("ok=%v body=%s, want an Invalid fid detail", ok, rec.Body)
	}
}
