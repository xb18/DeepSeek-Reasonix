package tool

import (
	"context"
	"slices"
	"testing"
)

type requiredTool struct {
	stubTool
	by string
}

func (r requiredTool) HostRequiredBy() string { return r.by }

func TestDisableToolsRemovesFromEverySurfaceAndBlocksLaterAdds(t *testing.T) {
	r := NewRegistry()
	r.Add(stubTool{name: "web_fetch"})
	r.Add(stubTool{name: "read_file"})

	if conflicts := r.DisableTools([]string{"web_fetch", "ghost"}); len(conflicts) != 0 {
		t.Fatalf("conflicts = %+v, want none", conflicts)
	}
	if _, ok := r.Get("web_fetch"); ok {
		t.Fatal("a disabled tool is still registered")
	}
	if got := namesOf(r.ProviderSchemas(context.Background())); !slices.Equal(got, []string{"read_file"}) {
		t.Fatalf("provider schemas = %v, want only read_file", got)
	}
	if _, name, _ := r.ResolveCall("web_fetch"); name != "" {
		t.Fatalf("a call resolved to the disabled tool %q", name)
	}
	r.Add(stubTool{name: "web_fetch"})
	if _, ok := r.Get("web_fetch"); ok {
		t.Fatal("a later registration path brought the disabled tool back")
	}
	refusal, ok := r.DisabledRefusal("web_fetch")
	if !ok || refusal.Code != CodeToolDisabled || refusal.Remedy == nil {
		t.Fatalf("refusal = %+v ok=%v, want %s with the setting that lifts it", refusal, ok, CodeToolDisabled)
	}
	if _, ok := r.DisabledRefusal("read_file"); ok {
		t.Fatal("an enabled tool answered with a disabled refusal")
	}
}

func TestDisableToolsKeepsWhatTheHostNeedsAndSaysWho(t *testing.T) {
	r := NewRegistry()
	r.Add(requiredTool{stubTool: stubTool{name: "todo_write"}, by: "task plans"})
	conflicts := r.DisableTools([]string{"todo_write"})
	if _, ok := r.Get("todo_write"); !ok {
		t.Fatal("a host-required tool was removed")
	}
	if len(conflicts) != 1 || conflicts[0].Tool != "todo_write" || conflicts[0].By != "task plans" {
		t.Fatalf("conflicts = %+v, want todo_write needed by task plans", conflicts)
	}
	if _, ok := r.DisabledRefusal("todo_write"); ok {
		t.Fatal("a kept tool answers as disabled")
	}
}

func TestDisableToolsBumpsTheSchemaRevisionOnlyWhenSomethingChanged(t *testing.T) {
	r := NewRegistry()
	r.Add(stubTool{name: "web_fetch"})
	before := r.SchemaRevision()
	r.DisableTools([]string{"ghost"})
	if r.SchemaRevision() != before {
		t.Fatal("naming a tool nobody registered changed the schema revision")
	}
	r.DisableTools([]string{"web_fetch"})
	if r.SchemaRevision() == before {
		t.Fatal("removing a tool did not change the schema revision")
	}
}
