package agent

import (
	"context"
	"strings"
	"testing"

	"reasonix/internal/contract/event"
	"reasonix/internal/contract/provider"
	"reasonix/internal/state/sessionstore"
)

// hookProvider runs hook on each model request, which is a moment the child is
// certainly inside a Run.
type hookProvider struct {
	*scriptedProvider
	hook func(call int)
}

func (h hookProvider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	h.hook(h.call)
	return h.scriptedProvider.Stream(ctx, req)
}

func steerableChild(t *testing.T, steer func(Steerer)) *scriptedProvider {
	t.Helper()
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{toolCallChunk("b1", "bash", `{"command":"echo one"}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "all done"}, {Type: provider.ChunkDone}},
	}}
	var steerer Steerer
	hooked := hookProvider{prov, func(call int) {
		if call == 0 && steerer != nil {
			steer(steerer)
		}
	}}
	_, err := RunSubAgentWithSession(context.Background(), hooked, evidenceRegistry(), sessionstore.NewSession("sys"), "do the thing",
		Options{SubagentDepth: 1, CheckTargetAccess: allowTestFileAccess, OnStart: func(s Steerer) { steerer = s }}, event.Discard)
	if err != nil {
		t.Fatalf("RunSubAgentWithSession: %v", err)
	}
	return prov
}

func lastRequestText(prov *scriptedProvider) string {
	var b strings.Builder
	for _, m := range prov.requests[len(prov.requests)-1].Messages {
		b.WriteString(m.Content + "\n")
	}
	return b.String()
}

func TestAChildOffersItselfForSteeringAndReadsGuidanceAtTheNextToolBoundary(t *testing.T) {
	var admitted bool
	prov := steerableChild(t, func(s Steerer) { admitted = s.Steer("prefer the faster path") })
	if !admitted {
		t.Fatal("a running child refused guidance")
	}
	got := lastRequestText(prov)
	if !strings.Contains(got, sessionstore.MidTurnSteerPrefix) || !strings.Contains(got, "prefer the faster path") {
		t.Fatalf("the child's next request did not carry the user's guidance:\n%s", got)
	}
}

func TestHostFramedGuidanceIsNotReadAsTheUserSpeaking(t *testing.T) {
	var admitted bool
	prov := steerableChild(t, func(s Steerer) { admitted = s.SteerHostNotice("From the parent: skip the tests") })
	if !admitted {
		t.Fatal("a running child refused guidance")
	}
	got := lastRequestText(prov)
	if !strings.Contains(got, sessionstore.HostNoticePrefix) || strings.Contains(got, sessionstore.MidTurnSteerPrefix) {
		t.Fatalf("host guidance carried the wrong framing:\n%s", got)
	}
}

func TestAFinishedChildRefusesGuidance(t *testing.T) {
	var steerer Steerer
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}}}}
	if _, err := RunSubAgentWithSession(context.Background(), prov, evidenceRegistry(), sessionstore.NewSession("sys"), "x",
		Options{SubagentDepth: 1, OnStart: func(s Steerer) { steerer = s }}, event.Discard); err != nil {
		t.Fatal(err)
	}
	if steerer == nil || steerer.Steer("too late") {
		t.Fatal("a child that has finished admitted guidance it can never read")
	}
}
