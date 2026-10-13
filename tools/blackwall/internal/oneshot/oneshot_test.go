package oneshot

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

type fakeClient struct{ agents []wazuh.Agent }

func (f *fakeClient) ListAgents(context.Context) ([]wazuh.Agent, error) { return f.agents, nil }

func TestPrint_TabulatesAgents(t *testing.T) {
	fc := &fakeClient{agents: []wazuh.Agent{
		{ID: "000", Name: "master", Status: "active"},
		{ID: "001", Name: "host-01", Status: "disconnected"},
	}}
	var buf bytes.Buffer
	if err := Print(context.Background(), &buf, fc); err != nil {
		t.Fatalf("Print: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"ID", "NAME", "STATUS", "000", "master", "active", "001", "disconnected"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
