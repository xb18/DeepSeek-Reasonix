package serve

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"reasonix/internal/contract/config"
	"reasonix/internal/session/control"
)

func TestSubagentGuidanceLimitsAreWrittenOnlyWithTheGrant(t *testing.T) {
	s := newProviderEditServer(t)
	srv := httptest.NewServer(operatorHandler(s))
	defer srv.Close()
	resp := postProvider(t, srv.URL, "/subagent-steer-limits", `{"maxBytes":1,"perMinute":1}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST without the grant = %d, want 403", resp.StatusCode)
	}
}

func TestSaveSubagentGuidanceLimitsPersistsBoth(t *testing.T) {
	s := newProviderEditServer(t)
	s.AllowProviderEdit()
	srv := httptest.NewServer(operatorHandler(s))
	defer srv.Close()

	got := readJSON[control.SubagentSteerSettings](t, srv.URL, "/subagent-steer-limits")
	if got.MaxBytes != 16384 || got.PerMinute != 30 || got.DefaultMaxBytes != 16384 || got.DefaultPerMinute != 30 {
		t.Fatalf("unset limits read %+v, want the named defaults", got)
	}
	for _, bad := range []string{`{}`, `{"maxBytes":10}`, `{"maxBytes":-1,"perMinute":3}`, `{"maxBytes":10,"perMinute":-2}`, `not json`} {
		resp := postProvider(t, srv.URL, "/subagent-steer-limits", bad)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("POST %q = %d, want 400", bad, resp.StatusCode)
		}
	}
	resp := postProvider(t, srv.URL, "/subagent-steer-limits", `{"maxBytes":4096,"perMinute":0}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := readAllString(resp)
		t.Fatalf("POST = %d: %s", resp.StatusCode, body)
	}
	if b, m := config.LoadForEdit(config.UserConfigPath()).SubagentSteerLimits(); b != 4096 || m != 0 {
		t.Fatalf("the user file holds %d and %d, want 4096 and 0 (lifted)", b, m)
	}
	if back := readJSON[control.SubagentSteerSettings](t, srv.URL, "/subagent-steer-limits"); back.MaxBytes != 4096 || back.PerMinute != 0 {
		t.Fatalf("read back = %+v", back)
	}
}
