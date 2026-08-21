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

package graph_test

import (
	"testing"
	"time"

	"github.com/RonsenbergVI/fraise/internal/graph"
)

// fakeHasher returns the material it was handed instead of hashing it, so a test
// can pin the exact bytes a node keys itself by rather than an opaque number.
type fakeHasher struct{}

func (fakeHasher) Hash(s string) string { return "H(" + s + ")" }
func (fakeHasher) Seed() uint64         { return 0 }

// TestNodeKeyMaterialIsTypeTagged pins the key material of every node kind. Each
// hashes its own type tag ahead of its value, which is what makes identity
// (type, value); the tags differ in their first byte, so no two namespaces can
// produce the same material. These strings are the contract — editing one moves
// every key of that type, and this test is where that shows up.
func TestNodeKeyMaterialIsTypeTagged(t *testing.T) {
	fact := graph.Fact[string]{NodeAttributes: graph.NodeAttributes{Value: "billing"}}
	topic := graph.Topic[string]{NodeAttributes: graph.NodeAttributes{Value: "billing"}}
	entity := graph.NamedEntity[string]{NodeAttributes: graph.NodeAttributes{Value: "billing"}}

	cases := []struct {
		name string
		node graph.Node[string]
		want string
	}{
		{"fact", fact, "H(fact:billing)"},
		{"topic", &topic, "H(topic:billing)"},
		{"named entity", &entity, "H(entity:billing)"},
		{"mentions", graph.Mentions[string]{Fact: &fact, NamedEntity: &entity}, "H(mentions:billing\x00billing)"},
		{"is about", graph.IsAbout[string]{Fact: &fact, Topic: &topic}, "H(isabout:billing\x00billing)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.node.Hash(fakeHasher{}); got != tc.want {
				t.Errorf("Hash() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestNodeKeysDistinguishTypesOfTheSameText is the contract the tags exist for,
// checked through the production hasher: five nodes that all read "billing" are
// five distinct keys. Sharing one is the ticket's failure — the second node of a
// colliding pair is never stored, and the edge between them closes on itself.
func TestNodeKeysDistinguishTypesOfTheSameText(t *testing.T) {
	g := newGraph()
	now := time.Now()

	fact := mkFact(g, "billing", now)
	topic := mkTopic(g, "billing", now)
	entity := mkEntity(g, "billing", now)

	nodes := []struct {
		name string
		node graph.Node[uint64]
	}{
		{"fact", fact},
		{"topic", topic},
		{"named entity", entity},
		{"mentions", graph.Mentions[uint64]{Fact: &fact, NamedEntity: entity, Hasher: g.GetHasher()}},
		{"is about", graph.IsAbout[uint64]{Fact: &fact, Topic: topic, Hasher: g.GetHasher()}},
	}

	seen := make(map[uint64]string, len(nodes))
	for _, n := range nodes {
		key := n.node.Key()
		if prior, dup := seen[key]; dup {
			t.Errorf("%s and %s both key to %d, want one key per (type, value)", prior, n.name, key)
		}
		seen[key] = n.name
	}
}

// TestFactKeyIgnoresProvenance separates the two identities a traceable memory
// has. What a fact *is* is its text: remembering one sentence from a contract
// and again from a call is one memory that two origins support, not two
// memories that happen to read alike. Folding the origin into the key would
// split it — two nodes, two sets of anchor edges, a recall that returns the
// same sentence twice — so provenance is an attribute and Fact.Hash must stay
// blind to it.
//
// Remember.Hash deliberately is *not* blind to it: that hash keys the plan
// cache, where reusing one origin's write plan for another's would persist the
// wrong provenance. The two are orthogonal on purpose, and this test is the
// half that fails loudly if someone "fixes" the asymmetry by making them agree.
func TestFactKeyIgnoresProvenance(t *testing.T) {
	const text = "acme moved to annual billing"
	h := &fakeHasher{}

	fromContract := graph.Fact[string]{
		NodeAttributes: graph.NodeAttributes{Value: text},
		Source:         "doc://contracts/acme-2026.pdf#p4",
		Hasher:         h,
	}
	fromCall := graph.Fact[string]{
		NodeAttributes: graph.NodeAttributes{Value: text},
		Source:         "session://2026-08-21/call-17",
		Hasher:         h,
	}
	unsourced := graph.Fact[string]{
		NodeAttributes: graph.NodeAttributes{Value: text},
		Hasher:         h,
	}

	if fromContract.Key() != fromCall.Key() {
		t.Errorf("one fact from two origins produced two keys (%v, %v): the memory would be split in two",
			fromContract.Key(), fromCall.Key())
	}
	if fromContract.Key() != unsourced.Key() {
		t.Errorf("adding a source changed a fact's key (%v vs %v): every fact stored before provenance existed would become unreachable",
			fromContract.Key(), unsourced.Key())
	}

	// The provenance is still readable off the node — Fact satisfies Sourced,
	// which is how the read path attaches it without knowing concrete types.
	var node graph.Node[string] = fromContract
	sourced, ok := node.(graph.Sourced)
	if !ok {
		t.Fatal("graph.Fact does not satisfy graph.Sourced: the read path cannot recover provenance")
	}
	if want := "doc://contracts/acme-2026.pdf#p4"; sourced.GetSource() != want {
		t.Errorf("GetSource() = %q, want %q", sourced.GetSource(), want)
	}

	// A topic reading the same text carries no provenance to recover, and must
	// not be forced to answer for one.
	var topic graph.Node[string] = &graph.Topic[string]{
		NodeAttributes: graph.NodeAttributes{Value: text},
		Hasher:         h,
	}
	if _, ok := topic.(graph.Sourced); ok {
		t.Error("graph.Topic satisfies Sourced: an anchor the graph derived would report an origin it never had")
	}
}
