package boot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"reasonix/internal/contract/provider"
)

// touchingProvider reads one file when a prompt asks it to, then answers. The
// session therefore touches a path the way a real run does: through a tool call.
type touchingProvider struct {
	mu   sync.Mutex
	reqs []provider.Request
}

func (p *touchingProvider) Name() string { return "boot-touching" }

func (p *touchingProvider) Stream(_ context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	p.mu.Lock()
	p.reqs = append(p.reqs, req)
	p.mu.Unlock()
	ch := make(chan provider.Chunk, 3)
	last := req.Messages[len(req.Messages)-1]
	if last.Role == provider.RoleUser && strings.Contains(last.Content, "touch-the-file") {
		ch <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "r-1", Name: "read_file", Arguments: `{"path":"src/a.ts"}`}}
	} else {
		ch <- provider.Chunk{Type: provider.ChunkText, Text: "ok"}
	}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	close(ch)
	return ch, nil
}

var (
	touchingOnce sync.Once
	touchingMu   sync.Mutex
	touchingCur  *touchingProvider
)

func registerTouching(p *touchingProvider) {
	touchingMu.Lock()
	touchingCur = p
	touchingMu.Unlock()
	touchingOnce.Do(func() {
		provider.Register("boot-touching", func(provider.Config) (provider.Provider, error) {
			touchingMu.Lock()
			defer touchingMu.Unlock()
			return touchingCur, nil
		})
	})
}

func writeRule(t *testing.T, dir, rel, body string) {
	t.Helper()
	path := filepath.Join(dir, ".reasonix", "rules", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Rules reach the model as turn-dynamic state through the real assembly: the
// unconditional ones once and again when they change, the path-scoped ones when
// the session has touched a matching file, both announced when they go, and the
// cache-stable prefix never moves.
func TestEffectRulesAreProjectedOntoTheTurnAndNeverIntoThePrefix(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	rec := &touchingProvider{}
	registerTouching(rec)
	writeFile(t, dir, "reasonix.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[codegraph]
enabled = false

[[providers]]
name = "test-model"
kind = "boot-touching"
model = "x"
`)
	writeFile(t, dir, "src/a.ts", "export const a = 1\n")
	writeRule(t, dir, "style.md", "Use tabs, never spaces.\n")
	writeRule(t, dir, "ts.md", "---\npaths:\n  - \"src/**/*.ts\"\n---\nTypeScript files use strict mode.\n")
	approveWorkspace(t, dir)

	ctrl, err := Build(context.Background(), Options{WorkspaceRoot: dir})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	turn := func(prompt string) provider.Request {
		t.Helper()
		rec.mu.Lock()
		start := len(rec.reqs)
		rec.mu.Unlock()
		if err := ctrl.Run(context.Background(), prompt); err != nil {
			t.Fatalf("Run(%q): %v", prompt, err)
		}
		rec.mu.Lock()
		defer rec.mu.Unlock()
		return rec.reqs[start]
	}

	first := turn("turn-one")
	proj := projectionOf(t, first, "turn-one")
	if block := blockOf(proj, "project-rules"); !strings.Contains(block, "Use tabs") {
		t.Fatalf("the first turn did not carry the unconditional rule:\n%s", proj)
	}
	if blockOf(proj, "scoped-rules") != "" || strings.Contains(proj, "strict mode") {
		t.Fatalf("a path-scoped rule arrived before any matching file was touched:\n%s", proj)
	}
	if again := blockOf(projectionOf(t, turn("turn-two"), "turn-two"), "project-rules"); again != "" {
		t.Fatalf("the rules were sent again although nothing changed:\n%s", again)
	}

	turn("touch-the-file")
	scoped := blockOf(projectionOf(t, turn("turn-after-touch"), "turn-after-touch"), "scoped-rules")
	if !strings.Contains(scoped, "strict mode") || !strings.Contains(scoped, "applies to src/**/*.ts") {
		t.Fatalf("a touched matching file did not bring its scoped rule:\n%s", scoped)
	}
	if again := blockOf(projectionOf(t, turn("turn-later"), "turn-later"), "scoped-rules"); again != "" {
		t.Fatalf("the scoped rule was sent twice:\n%s", again)
	}

	writeRule(t, dir, "style.md", "Use tabs, never spaces.\nKeep lines short.\n")
	if changed := blockOf(projectionOf(t, turn("turn-edited"), "turn-edited"), "project-rules"); !strings.Contains(changed, "Keep lines short") {
		t.Fatalf("an edited rule never reached the model:\n%s", changed)
	}
	if err := os.Remove(filepath.Join(dir, ".reasonix", "rules", "ts.md")); err != nil {
		t.Fatal(err)
	}
	gone := blockOf(projectionOf(t, turn("turn-removed"), "turn-removed"), "scoped-rules")
	if gone == "" || strings.Contains(gone, "strict mode") {
		t.Fatalf("a removed scoped rule kept standing as current:\n%s", gone)
	}

	if a, b := systemOf(first), systemOf(turn("turn-last")); a != b {
		t.Fatalf("editing rules moved the cache-stable prefix:\nfirst difference: %q", firstDivergence(a, b))
	}
}
