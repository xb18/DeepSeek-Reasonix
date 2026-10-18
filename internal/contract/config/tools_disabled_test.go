package config

import (
	"slices"
	"strings"
	"testing"
)

func TestUserFileDisablesToolsByName(t *testing.T) {
	cfg, _ := loadScoped(t, "[tools]\ndisabled = [\"web_fetch\", \"ask\"]\n", "")
	if got := cfg.DisabledTools(); !slices.Equal(got, []string{"ask", "web_fetch"}) {
		t.Fatalf("DisabledTools = %v, want the two names sorted", got)
	}
}

func TestProjectCanOnlyAddToTheDisabledTools(t *testing.T) {
	cfg, _ := loadScoped(t, "[tools]\ndisabled = [\"web_fetch\"]\n", "[tools]\ndisabled = [\"ask\"]\n")
	if got := cfg.DisabledTools(); !slices.Equal(got, []string{"ask", "web_fetch"}) {
		t.Fatalf("DisabledTools = %v, want the user's and the project's names together", got)
	}
	cfg, _ = loadScoped(t, "[tools]\ndisabled = [\"web_fetch\"]\n", "[tools]\ndisabled = []\n")
	if got := cfg.DisabledTools(); !slices.Equal(got, []string{"web_fetch"}) {
		t.Fatalf("a project file took a disabled tool back: %v", got)
	}
}

func TestDisabledToolsAreTrimmedAndUnique(t *testing.T) {
	cfg := Default()
	cfg.Tools.Disabled = []string{" web_fetch ", "web_fetch", "", "ask"}
	if got := cfg.DisabledTools(); !slices.Equal(got, []string{"ask", "web_fetch"}) {
		t.Fatalf("DisabledTools = %v", got)
	}
	var none *Config
	if len(none.DisabledTools()) != 0 {
		t.Fatal("a nil config disables tools")
	}
}

func TestDisabledToolsRenderOnlyWhenSet(t *testing.T) {
	if out := RenderTOMLForScope(Default(), RenderScopeUser); strings.Contains(out, "\ndisabled = ") {
		t.Fatalf("the default rendered an active disabled list:\n%s", out)
	}
	cfg := Default()
	cfg.Tools.Disabled = []string{"web_fetch"}
	if out := RenderTOMLForScope(cfg, RenderScopeUser); !strings.Contains(out, `disabled = ["web_fetch"]`) {
		t.Fatalf("a chosen list did not render:\n%s", out)
	}
}

func TestSetToolDisabledEditsTheListInPlace(t *testing.T) {
	cfg := Default()
	cfg.SetToolDisabled("web_fetch", true)
	cfg.SetToolDisabled("ask", true)
	cfg.SetToolDisabled("web_fetch", true)
	if got := cfg.DisabledTools(); !slices.Equal(got, []string{"ask", "web_fetch"}) {
		t.Fatalf("after two disables = %v", got)
	}
	cfg.SetToolDisabled("ask", false)
	if got := cfg.DisabledTools(); !slices.Equal(got, []string{"web_fetch"}) {
		t.Fatalf("after re-enabling ask = %v", got)
	}
}
