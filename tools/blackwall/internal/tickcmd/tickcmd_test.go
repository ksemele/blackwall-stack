package tickcmd

import (
	"testing"
	"time"
)

func TestEvery_EmitsTickMsg(t *testing.T) {
	cmd := Every(5 * time.Millisecond)
	msg := cmd()
	if _, ok := msg.(TickMsg); !ok {
		t.Errorf("msg type = %T, want TickMsg", msg)
	}
}
