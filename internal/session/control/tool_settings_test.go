package control

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"reasonix/internal/base/testenv"
	"reasonix/internal/contract/config"
	"reasonix/internal/contract/tool"
)

type settingTool struct {
	name, desc, needed string
}

func (s settingTool) Name() string            { return s.name }
func (s settingTool) Description() string     { return s.desc }
func (s settingTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (s settingTool) Execute(context.Context, json.RawMessage) (string, error) {
	return "", nil
}
func (s settingTool) ReadOnly() bool { return true }

type neededTool struct{ settingTool }

func (n neededTool) HostRequiredBy() string { return n.needed }

func toolSettingNames(s ToolSettings) []string {
	var out []string
	for _, t := range s.Tools {
		out = append(out, t.Name)
	}
	return out
}

func TestToolSettingsListWhatTheRequestCarriesAndWhatTheUserRemoved(t *testing.T) {
	t.Setenv("REASONIX_HOME", testenv.TempDir(t))
	reg := tool.NewRegistry()
	reg.Add(settingTool{name: "web_fetch", desc: "Fetch a page.\nSecond line."})
	reg.Add(neededTool{settingTool{name: "use_capability", desc: "Reach a deferred tool.", needed: tool.FeatureDeferredMCP}})
	c := New(Options{Registry: reg})

	if err := c.SaveToolDisabled("web_fetch", true); err != nil {
		t.Fatalf("save: %v", err)
	}
	if !slices.Contains(config.LoadForEdit(config.UserConfigPath()).DisabledTools(), "web_fetch") {
		t.Fatal("the removal did not reach the user file")
	}
	got := c.ToolSettings()
	if !slices.Equal(toolSettingNames(got), []string{"use_capability", "web_fetch"}) {
		t.Fatalf("tools = %v, want the carried one and the removed one", toolSettingNames(got))
	}
	for _, s := range got.Tools {
		switch s.Name {
		case "web_fetch":
			if !s.Disabled {
				t.Fatal("web_fetch is not marked disabled")
			}
		case "use_capability":
			if s.Disabled || s.RequiredBy != tool.FeatureDeferredMCP {
				t.Fatalf("use_capability = %+v, want enabled with who needs it", s)
			}
		}
	}
	if err := c.SaveToolDisabled("web_fetch", false); err != nil {
		t.Fatalf("re-enable: %v", err)
	}
	if slices.Contains(config.LoadForEdit(config.UserConfigPath()).DisabledTools(), "web_fetch") {
		t.Fatal("re-enabling did not reach the user file")
	}
}

func TestSaveToolDisabledRefusesWhatTheHostNeeds(t *testing.T) {
	t.Setenv("REASONIX_HOME", testenv.TempDir(t))
	reg := tool.NewRegistry()
	reg.Add(neededTool{settingTool{name: "use_capability", needed: tool.FeatureDeferredMCP}})
	c := New(Options{Registry: reg})
	err := c.SaveToolDisabled("use_capability", true)
	var required *ToolRequiredError
	if !errors.As(err, &required) || required.Tool != "use_capability" || required.By != tool.FeatureDeferredMCP {
		t.Fatalf("err = %v, want a ToolRequiredError naming who needs it", err)
	}
	if len(config.LoadForEdit(config.UserConfigPath()).DisabledTools()) != 0 {
		t.Fatal("a refused removal reached the user file")
	}
}
