// MIT License

// Copyright (c) 2026 René-Jean Corneille

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package query

import (
	"testing"

	"github.com/RonsenbergVI/fraise/internal/containers"
)

func TestRememberIsWrite(t *testing.T) {
	var r Remember[string, float32]
	if !r.IsWrite() {
		t.Error("Remember.IsWrite() = false, want true")
	}
}

// Unlike Recall, Remember.SetGraphID has a pointer receiver, so the update
// persists when called through a pointer.
func TestRememberSetGraphIDPersists(t *testing.T) {
	r := &Remember[string, float32]{}
	r.SetGraphID(7)
	if got := r.GetGraphID(); got != 7 {
		t.Errorf("after SetGraphID(7), GetGraphID() = %d, want 7", got)
	}
}

func TestRememberGetGraphID(t *testing.T) {
	r := Remember[string, float32]{context: QueryContext{GraphID: 3}}
	if got := r.GetGraphID(); got != 3 {
		t.Errorf("GetGraphID() = %d, want 3", got)
	}
}

func TestRememberHash(t *testing.T) {
	r := Remember[string, float32]{
		Value:    "hello world",
		Entities: []string{"alice"},
		Topics:   []string{"greeting"},
		Vector:   containers.NewVector[string]([]float32{0.5}),
		context:  QueryContext{GraphID: 2},
	}
	h := &fakeHasher{}

	// Hash folds in graph, value, the delimited entity/topic lists and the bound
	// vector. A non-empty provenance reference adds its own tagged segment;
	// omitting that segment here preserves the cache identity of legacy writes.
	const want = "g=2|v=hello world|en=alice|to=greeting|vec=H(0x1p-01)"
	if got := r.Hash(h); got != "H("+want+")" {
		t.Errorf("Hash() = %q, want %q", got, "H("+want+")")
	}
	if h.last != want {
		t.Errorf("hasher received %q, want %q", h.last, want)
	}
}

// TestRememberHashDistinguishesGraphAndTags is the real contract: writes that
// differ only in graph, entities, topics, provenance or the bound vector must
// not share a cache key, or the engine reuses a stale plan and writes to the
// wrong graph/tags (or with the wrong embedding, or the wrong origin).
func TestRememberHashDistinguishesGraphAndTags(t *testing.T) {
	base := func() Remember[string, float32] {
		return Remember[string, float32]{Value: "the parrot is turquoise"}
	}
	variants := map[string]Remember[string, float32]{
		"base":   base(),
		"graph":  func() Remember[string, float32] { r := base(); r.context.GraphID = 5; return r }(),
		"topic":  func() Remember[string, float32] { r := base(); r.Topics = []string{"birds"}; return r }(),
		"entity": func() Remember[string, float32] { r := base(); r.Entities = []string{"polly"}; return r }(),
		"source": func() Remember[string, float32] { r := base(); r.Source = "doc://aviary-log#12"; return r }(),
		"vector-a": func() Remember[string, float32] {
			r := base()
			r.Vector = containers.NewVector[string]([]float32{1, 0})
			return r
		}(),
		"vector-b": func() Remember[string, float32] {
			r := base()
			r.Vector = containers.NewVector[string]([]float32{0, 1})
			return r
		}(),
	}

	seen := make(map[string]string)
	for name, r := range variants {
		key := r.Hash(&fakeHasher{})
		if other, clash := seen[key]; clash {
			t.Errorf("hash collision: %q and %q both produced %q", name, other, key)
		}
		seen[key] = name
	}
}

// TestRememberHashSeparatesSourcesOfOneFact is the provenance regression, and
// it is the case the two hashes are easiest to confuse. The *fact* is
// content-addressed by its text, so remembering one sentence from two origins
// is deliberately one node — but the two *writes* are different writes, and
// the plan cache keys on this hash. Were Source left out, the second remember
// would hit the first's cached plan and persist the first's origin: the graph
// would then name the contract as the source of something the call said, with
// no error raised anywhere. A traceability feature that silently records the
// wrong origin is worse than one that records none.
func TestRememberHashSeparatesSourcesOfOneFact(t *testing.T) {
	const fact = "acme moved to annual billing"

	fromContract := Remember[string, float32]{Value: fact, Source: "doc://contracts/acme-2026.pdf#p4"}
	fromCall := Remember[string, float32]{Value: fact, Source: "session://2026-08-21/call-17"}
	unsourced := Remember[string, float32]{Value: fact}

	contract, call, none := fromContract.Hash(&fakeHasher{}), fromCall.Hash(&fakeHasher{}), unsourced.Hash(&fakeHasher{})

	if contract == call {
		t.Errorf("two origins of one fact share a plan-cache key (%q): the second write would persist the first's provenance", contract)
	}
	if contract == none || call == none {
		t.Error("a sourced remember shares a plan-cache key with an unsourced one: the cached plan would write the wrong provenance")
	}
}

func TestRememberPlan(t *testing.T) {
	var r Remember[string, float32]
	s, err := r.Plan(nil)
	if err != nil {
		t.Fatalf("Plan() err = %v, want nil", err)
	}
	if s == nil {
		t.Fatal("Plan() stream = nil, want a ready stream")
	}
	if s.Query != Query[string, float32](&r) {
		t.Errorf("Plan() stream.Query = %v, want the receiver", s.Query)
	}
	select {
	case <-s.Done():
		t.Error("Plan() stream is already done; it must stay open until committed")
	default:
	}
}
