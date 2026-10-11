package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"reasonix/internal/contract/event"
	"reasonix/internal/contract/provider"
	"reasonix/internal/contract/tool"
	"reasonix/internal/ext/extension"
	"reasonix/internal/ext/extension/dispatch"
	"reasonix/internal/ext/extension/protocol"
	"reasonix/internal/runtime/taskpolicy"
	"reasonix/internal/state/sessionstore"
)

type namedFileWriter struct{ root string }

func (namedFileWriter) Name() string            { return "write_file" }
func (namedFileWriter) Description() string     { return "" }
func (namedFileWriter) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (namedFileWriter) ReadOnly() bool          { return false }
func (namedFileWriter) WritesNamedPaths() bool  { return true }
func (w namedFileWriter) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var p struct{ Path, Content string }
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	return "wrote", os.WriteFile(filepath.Join(w.root, p.Path), []byte(p.Content), 0o600)
}

func TestToolAfterRewriteCannotHideWrittenCodeFromProseWaiver(t *testing.T) {
	for _, mode := range []string{"block", "sidecar error"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			for name, body := range map[string]string{"Program.cs": "class P {}\n", "README.md": "readme\n"} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			client := &fakeDispatchClient{interceptFn: func(ev protocol.InterceptEvent, payload json.RawMessage) (protocol.InterceptResult, error) {
				if ev == protocol.EventToolAfter && strings.Contains(string(payload), "Program.cs") {
					if mode == "block" {
						return blockWith("lint rejected the write"), nil
					}
					return protocol.InterceptResult{}, errors.New("sidecar timeout")
				}
				return protocol.InterceptResult{Decision: protocol.DecisionContinue}, nil
			}}
			d := newExtDispatcher(client, true, nil, extension.PointToolAfter)
			reg := tool.NewRegistry()
			reg.Add(namedFileWriter{root: root})
			reg.Add(fakeTool{name: "bash"})
			a := New(nil, reg, sessionstore.NewSession(""), Options{Extensions: d}, event.Discard)
			a.observeRoot = root
			a.writeWorkspaceRoot = root
			a.executeOne(context.Background(), &a.turn, provider.ToolCall{ID: "1", Name: "write_file", Arguments: `{"path":"Program.cs","content":"class Q {}\n"}`})
			a.executeOne(context.Background(), &a.turn, provider.ToolCall{ID: "2", Name: "write_file", Arguments: `{"path":"README.md","content":"note\n"}`})
			a.turn.policySet = true
			a.turn.policy = taskpolicy.TaskPolicy{Verification: taskpolicy.VerifyTargeted}
			src, _ := os.ReadFile(filepath.Join(root, "Program.cs"))
			check := a.finalReadinessCheckFor()
			if string(src) != "class Q {}\n" {
				t.Fatalf("Program.cs = %q, the write did not land", src)
			}
			if check.missingVerification != 1 || !strings.Contains(check.reason, "Program.cs") {
				t.Errorf("missing verification = %d, reason %q; want the written code to keep the check owed", check.missingVerification, check.reason)
			}
		})
	}
}

func TestToolAfterRewriteCannotHideTheOnlyWrittenCode(t *testing.T) {
	for _, mode := range []string{"block", "sidecar error"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			client := &fakeDispatchClient{interceptFn: func(ev protocol.InterceptEvent, _ json.RawMessage) (protocol.InterceptResult, error) {
				if ev != protocol.EventToolAfter {
					return protocol.InterceptResult{Decision: protocol.DecisionContinue}, nil
				}
				if mode == "block" {
					return blockWith("lint rejected the write"), nil
				}
				return protocol.InterceptResult{}, errors.New("sidecar timeout")
			}}
			d := newExtDispatcher(client, true, nil, extension.PointToolAfter)
			reg := tool.NewRegistry()
			reg.Add(namedFileWriter{root: root})
			reg.Add(fakeTool{name: "bash"})
			a := New(nil, reg, sessionstore.NewSession(""), Options{Extensions: d}, event.Discard)
			a.observeRoot = root
			a.writeWorkspaceRoot = root
			a.executeOne(context.Background(), &a.turn, provider.ToolCall{ID: "1", Name: "write_file", Arguments: `{"path":"Program.cs","content":"class Q {}\n"}`})
			a.turn.policySet = true
			a.turn.policy = taskpolicy.TaskPolicy{Verification: taskpolicy.VerifyTargeted}
			if _, err := os.Stat(filepath.Join(root, "Program.cs")); err != nil {
				t.Fatalf("the write did not land: %v", err)
			}
			check := a.finalReadinessCheckFor()
			if check.missingVerification != 1 {
				t.Errorf("missing verification = %d, reason %q; want the only written code to keep the check owed", check.missingVerification, check.reason)
			}
		})
	}
}

func TestProseNamedWriteLinkTargets(t *testing.T) {
	cases := []struct {
		name       string
		rootIsLink bool
		linkTo     func(root, outside string) string
		blockAfter bool
		wantOwed   bool
	}{
		{name: "plain path adds nothing", wantOwed: false},
		{name: "workspace root is itself a link", rootIsLink: true, wantOwed: false},
		{name: "link to code, removed", linkTo: func(root, _ string) string { return filepath.Join(root, "Program.cs") }, wantOwed: true},
		{name: "link to code outside the workspace", linkTo: func(_, outside string) string { return filepath.Join(outside, "Program.cs") }, wantOwed: true},
		{name: "link to code, write reported failed", linkTo: func(root, _ string) string { return filepath.Join(root, "Program.cs") }, blockAfter: true, wantOwed: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			real := t.TempDir()
			outside := t.TempDir()
			root := real
			if tc.rootIsLink {
				root = filepath.Join(t.TempDir(), "ws")
				if err := os.Symlink(real, root); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			for _, dir := range []string{real, outside} {
				if err := os.WriteFile(filepath.Join(dir, "Program.cs"), []byte("class P {}\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			link := filepath.Join(root, "NOTES.md")
			if tc.linkTo != nil {
				if err := os.Symlink(tc.linkTo(real, outside), link); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			var d *dispatch.Dispatcher
			if tc.blockAfter {
				client := &fakeDispatchClient{interceptFn: func(ev protocol.InterceptEvent, _ json.RawMessage) (protocol.InterceptResult, error) {
					if ev == protocol.EventToolAfter {
						return blockWith("lint rejected the write"), nil
					}
					return protocol.InterceptResult{Decision: protocol.DecisionContinue}, nil
				}}
				d = newExtDispatcher(client, true, nil, extension.PointToolAfter)
			}
			reg := tool.NewRegistry()
			reg.Add(namedFileWriter{root: root})
			reg.Add(fakeTool{name: "bash"})
			a := New(nil, reg, sessionstore.NewSession(""), Options{Extensions: d}, event.Discard)
			a.observeRoot = root
			a.writeWorkspaceRoot = root
			a.executeOne(context.Background(), &a.turn, provider.ToolCall{ID: "1", Name: "write_file", Arguments: `{"path":"NOTES.md","content":"note\n"}`})
			if tc.linkTo != nil {
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
				a.executeOne(context.Background(), &a.turn, provider.ToolCall{ID: "2", Name: "bash", Arguments: `{"command":"rm NOTES.md"}`})
			}
			beyond, _ := a.task.ledger.MutationPathsBeyondProse(a.checkContract())
			owed := slices.ContainsFunc(beyond, func(p string) bool { return filepath.Base(p) == "Program.cs" })
			if owed != tc.wantOwed {
				t.Errorf("changed paths beyond prose = %v; want the code behind the link named = %v", beyond, tc.wantOwed)
			}
		})
	}
}
