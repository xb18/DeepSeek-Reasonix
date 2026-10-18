package serve

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"reasonix/internal/base/testenv"
	"reasonix/internal/contract/config"
	"reasonix/internal/contract/tool"
	"reasonix/internal/session/control"
)

type listedTool struct{ name string }

func (l listedTool) Name() string            { return l.name }
func (l listedTool) Description() string     { return l.name + " does a thing" }
func (l listedTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (l listedTool) Execute(context.Context, json.RawMessage) (string, error) {
	return "", nil
}
func (l listedTool) ReadOnly() bool { return true }

type neededListedTool struct{ listedTool }

func (neededListedTool) HostRequiredBy() string { return tool.FeatureGoalMode }

func newToolSettingsServer(t *testing.T) *Server {
	t.Helper()
	t.Setenv("REASONIX_HOME", testenv.TempDir(t))
	reg := tool.NewRegistry()
	reg.Add(listedTool{"web_fetch"})
	reg.Add(neededListedTool{listedTool{"update_goal"}})
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc, Registry: reg, SessionDir: testenv.TempDir(t)})
	t.Cleanup(ctrl.Close)
	closeSharedCatalogsOnCleanup(t)
	return New(ctrl, bc, config.ServeConfig{})
}

func TestToolSettingsListSaveAndRefuse(t *testing.T) {
	s := newToolSettingsServer(t)
	s.AllowProviderEdit()
	srv := httptest.NewServer(operatorHandler(s))
	defer srv.Close()

	got := readJSON[control.ToolSettings](t, srv.URL, "/tools")
	if len(got.Tools) != 2 || got.Tools[1].Name != "web_fetch" || got.Tools[0].RequiredBy != tool.FeatureGoalMode {
		t.Fatalf("GET /tools = %+v, want update_goal (needed) and web_fetch", got.Tools)
	}
	for body, want := range map[string]int{`{}`: http.StatusBadRequest, `{"name":"web_fetch"}`: http.StatusBadRequest, `{"name":"update_goal","disabled":true}`: http.StatusConflict} {
		resp := postProvider(t, srv.URL, "/tools/disabled", body)
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("POST /tools/disabled %s = %d (%s), want %d", body, resp.StatusCode, raw, want)
		}
	}
	resp := postProvider(t, srv.URL, "/tools/disabled", `{"name":"web_fetch","disabled":true}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /tools/disabled = %d", resp.StatusCode)
	}
	if !slices.Contains(config.LoadForEdit(config.UserConfigPath()).DisabledTools(), "web_fetch") {
		t.Fatal("the removal did not reach the user file")
	}
	if after := readJSON[control.ToolSettings](t, srv.URL, "/tools"); !after.Tools[1].Disabled {
		t.Fatalf("GET /tools after the save = %+v, want web_fetch disabled", after.Tools)
	}
}

func TestToolSettingsAreRefusedWithoutTheEditGrant(t *testing.T) {
	s := newToolSettingsServer(t)
	srv := httptest.NewServer(operatorHandler(s))
	defer srv.Close()
	resp := postProvider(t, srv.URL, "/tools/disabled", `{"name":"web_fetch","disabled":true}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST /tools/disabled without the grant = %d, want 403", resp.StatusCode)
	}
	if len(config.LoadForEdit(config.UserConfigPath()).DisabledTools()) != 0 {
		t.Fatal("a refused save reached the user file")
	}
}
