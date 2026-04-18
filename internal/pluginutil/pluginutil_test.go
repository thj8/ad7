package pluginutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid 32-char id", "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4", false},
		{"empty string", "", true},
		{"too short 31 chars", "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d", true},
		{"too long 33 chars", "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ParseID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}
	WriteJSON(w, http.StatusOK, data)

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", w.Header().Get("Content-Type"))
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["key"] != "value" {
		t.Errorf("response key = %q, want value", got["key"])
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusBadRequest, "bad request")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	body := strings.TrimSpace(w.Body.String())
	want := `{"error":"bad request"}`
	if body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
}
