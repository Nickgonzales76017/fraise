package query

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/RonsenbergVI/fraise/internal/graph"
)

func TestRememberHashDistinguishesSourceWithoutChangingLegacyEmptyHash(t *testing.T) {
	base := Remember[string, float32]{Value: "fact"}
	a, b := base, base
	a.Source, b.Source = "session:a", "session:b"
	if a.Hash(&fakeHasher{}) == b.Hash(&fakeHasher{}) {
		t.Fatal("different provenance references shared a plan-cache key")
	}
	if legacy := base.Hash(&fakeHasher{}); strings.Contains(legacy, "|src=") {
		t.Fatalf("empty provenance changed legacy cache identity: %q", legacy)
	}
}

func TestHitSourceAppearsOnlyInExplainMode(t *testing.T) {
	var node graph.Node[string] = graph.Fact[string]{
		NodeAttributes: graph.NodeAttributes{Value: "fact", Timestamp: time.Unix(1, 0).UTC()},
		Source:         "session:abc",
	}
	plain, err := json.Marshal(Hit[string, float32]{Node: &node, Score: 1})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), `"source"`) {
		t.Fatalf("plain recall leaked provenance: %s", plain)
	}
	explained, err := json.Marshal(Hit[string, float32]{
		Node:          &node,
		Score:         1,
		Source:        "session:abc",
		Contributions: []HitContribution[float32]{{Source: "text", Score: 1, Rank: 0, Count: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(explained), `"source":"session:abc"`) {
		t.Fatalf("explained recall omitted provenance: %s", explained)
	}
}
