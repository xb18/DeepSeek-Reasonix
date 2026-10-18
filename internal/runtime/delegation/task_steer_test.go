package delegation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"reasonix/internal/base/testenv"
	"reasonix/internal/contract/provider"
	"reasonix/internal/runtime/agent"
)

// steerTask is a task tool whose store holds one sub-agent run this process is
// executing, with a child that takes guidance.
func steerTask(t *testing.T, limits SteerLimits) (*TaskTool, context.Context, *SubagentRun, *fakeSteerer) {
	t.Helper()
	store := NewSubagentStore(testenv.TempDir(t) + "/subagents")
	store.WithSteerLimits(func() SteerLimits { return limits })
	root := testenv.TempDir(t)
	spec := testSubagentSpec(t, "worker")
	spec.Kind, spec.WorkspaceRoot = "task", root
	run := startedRun(t, store, spec, said(provider.RoleUser, "task"))
	t.Cleanup(run.Release)
	st := &fakeSteerer{open: true}
	store.attachSteerer(run.Session, st)
	task := NewTaskTool(&scriptedProvider{name: "p", turns: [][]provider.Chunk{{{Type: provider.ChunkText, Text: "x"}, {Type: provider.ChunkDone}}}},
		nil, nil, 20, 0, 0, 0, 0.0, "", "sys", nil, 0, "", "", nil).WithTranscripts(store, root, "base", "high")
	return task, agent.WithParentSession(context.Background(), spec.ParentSession), run, st
}

func taskCall(t *testing.T, task *TaskTool, ctx context.Context, args map[string]any) (string, error) {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	return task.Execute(ctx, raw)
}

func TestTaskWithATaskIdGivesTheRunningChildGuidanceInsteadOfStartingOne(t *testing.T) {
	task, ctx, run, st := steerTask(t, SteerLimits{})
	out, err := taskCall(t, task, ctx, map[string]any{"prompt": "skip the slow tests", "task_id": run.Ref})
	if err != nil {
		t.Fatalf("task_id call: %v", err)
	}
	if !strings.Contains(out, run.Ref) || !strings.Contains(strings.ToLower(out), "guidance") {
		t.Fatalf("result = %q, want it to say guidance reached %s", out, run.Ref)
	}
	if len(st.host) != 1 || !strings.Contains(st.host[0], "skip the slow tests") || !strings.HasPrefix(st.host[0], "From the parent session") {
		t.Fatalf("child got %q, want parent-framed guidance", st.host)
	}
	if len(st.user) != 0 {
		t.Fatalf("model guidance was framed as the user's: %q", st.user)
	}
}

func TestTaskIdRefusalsKeepTheirIdentityAcrossTheModelBoundary(t *testing.T) {
	task, ctx, run, _ := steerTask(t, SteerLimits{MaxBytes: 4})
	for _, tc := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"prompt": "this is too long", "task_id": run.Ref}, "subagent.steer_too_large"},
		{map[string]any{"prompt": "hi", "task_id": "sa_0000000000000000"}, "subagent.not_found"},
	} {
		_, err := taskCall(t, task, ctx, tc.args)
		if err == nil || !strings.HasPrefix(err.Error(), tc.want) {
			t.Fatalf("%v = %v, want an error leading with %s", tc.args, err, tc.want)
		}
	}
	other := agent.WithParentSession(context.Background(), "someone-else")
	if _, err := taskCall(t, task, other, map[string]any{"prompt": "hi", "task_id": run.Ref}); err == nil || !strings.HasPrefix(err.Error(), "subagent.not_owned") {
		t.Fatalf("another conversation = %v, want subagent.not_owned", err)
	}
}

func TestTaskIdOfAFinishedRunContinuesItLikeContinueFrom(t *testing.T) {
	task, ctx, run, _ := steerTask(t, SteerLimits{})
	if err := task.transcripts.SaveCompleted(run); err != nil {
		t.Fatal(err)
	}
	run.Release()
	_, err := taskCall(t, task, ctx, map[string]any{"prompt": "now add tests", "task_id": run.Ref})
	var notOwned, notFound bool
	if err != nil {
		notOwned = errors.Is(err, ErrSubagentNotOwned)
		notFound = errors.Is(err, ErrSubagentNotFound)
	}
	if notOwned || notFound || (err != nil && strings.HasPrefix(err.Error(), "subagent.")) {
		t.Fatalf("a finished run was refused as guidance (%v); it should continue as continue_from does", err)
	}
}

func TestTaskSchemaDescribesTaskId(t *testing.T) {
	task, _, _, _ := steerTask(t, SteerLimits{})
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(task.Schema(), &schema); err != nil {
		t.Fatal(err)
	}
	if _, ok := schema.Properties["task_id"]; !ok {
		t.Fatal("task_id is not in the schema, so the model cannot pass it")
	}
}

func TestTaskIdOfARunThatIsStartingIsRefusedAndStartsNothing(t *testing.T) {
	task, ctx, run, st := steerTask(t, SteerLimits{})
	// Registered as running, but its agent has not handed itself over yet.
	task.transcripts.attachSteerer(run.Session, nil)
	_, err := taskCall(t, task, ctx, map[string]any{"prompt": "go faster", "task_id": run.Ref})
	if err == nil || !strings.HasPrefix(err.Error(), "subagent.settling") || !errors.Is(err, ErrSubagentSettling) {
		t.Fatalf("task_id on a starting run = %v, want subagent.settling", err)
	}
	if len(st.host)+len(st.user) != 0 {
		t.Fatalf("guidance reached a child that was not ready: %v %v", st.host, st.user)
	}
	if !task.transcripts.held(run.Ref) {
		t.Fatal("the run lost its hold: a second writer could take the transcript")
	}
}

func TestTaskIdOfARunThatIsFinishingIsRefusedToo(t *testing.T) {
	task, ctx, run, st := steerTask(t, SteerLimits{})
	st.open = false
	_, err := taskCall(t, task, ctx, map[string]any{"prompt": "one more thing", "task_id": run.Ref})
	if err == nil || !errors.Is(err, ErrSubagentSettling) {
		t.Fatalf("task_id on a finishing run = %v, want subagent.settling", err)
	}
}
