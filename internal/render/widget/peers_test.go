package widget

import (
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

func TestPeersWidget_ZeroReturnsEmpty(t *testing.T) {
	ctx := &model.RenderContext{PeerCount: 0}
	cfg := defaultCfg()

	if got := Peers(ctx, cfg); !got.IsEmpty() {
		t.Errorf("Peers with zero count: expected empty, got %q", got.Text)
	}
}

func TestPeersWidget_NonZeroRendersCount(t *testing.T) {
	ctx := &model.RenderContext{PeerCount: 2}
	cfg := defaultCfg()

	got := Peers(ctx, cfg)
	if got.PlainText != "×2" {
		t.Errorf("Peers: expected %q, got %q", "×2", got.PlainText)
	}
}

func TestPeersWidget_RegisteredInRegistry(t *testing.T) {
	if _, ok := Registry["peers"]; !ok {
		t.Error("Registry missing 'peers' widget")
	}
}
