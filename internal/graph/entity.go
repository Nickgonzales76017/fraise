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

package graph

import (
	"time"

	"github.com/RonsenbergVI/fraise/internal/hash"
)

type Fact[K comparable] struct {
	NodeAttributes

	// Source is the fact's provenance: a bounded reference to where it came
	// from (a document, a session, a tool call), or empty for a fact recorded
	// without one. It sits on Fact rather than on NodeAttributes because only
	// a fact has an origin — a topic or an entity is an anchor this graph
	// derived, not something it was told.
	//
	// It is an attribute, never identity: see Hash. A fact is content-
	// addressed by its text, so two remembers of the same sentence from two
	// origins are one node, and the later write refreshes the provenance the
	// same way it refreshes the timestamp and the vector.
	Source string

	Hasher hash.Hasher[K, string] `json:"-"`
}

func (f Fact[K]) Key() K {
	return f.Hash(f.Hasher)
}

func (f Fact[K]) GetValue() string {
	return f.Value
}

func (f Fact[K]) GetTimestamp() time.Time {
	return f.Timestamp
}

func (f Fact[K]) GetAttributes() *NodeAttributes {
	return &f.NodeAttributes
}

// GetSource returns the fact's provenance reference, satisfying Sourced.
func (f Fact[K]) GetSource() string {
	return f.Source
}

// Hash keys the fact by its text in the fact namespace, so a topic or entity
// reading the same text is a different node (see Node).
//
// Source is deliberately absent. Provenance is what the graph knows *about* a
// fact, not part of which fact it is: folding it in would make 'acme moved to
// annual billing' remembered from a contract and from a call two separate
// nodes, splitting one memory in two and doubling every anchor that funds it.
// Remember.Hash does fold Source in — that is the plan cache, which must not
// reuse one origin's write plan for another's — and the two hashes answer
// different questions on purpose.
func (f Fact[K]) Hash(h hash.Hasher[K, string]) K {
	return h.Hash("fact:" + f.Value)
}

type NamedEntity[K comparable] struct {
	NodeAttributes

	Hasher hash.Hasher[K, string] `json:"-"`
}

func (n NamedEntity[K]) Key() K {
	return n.Hash(n.Hasher)
}

func (n NamedEntity[K]) GetValue() string {
	return n.Value
}

func (n NamedEntity[K]) GetTimestamp() time.Time {
	return n.Timestamp
}

func (n *NamedEntity[K]) GetAttributes() *NodeAttributes {
	return &n.NodeAttributes
}

// Hash keys the entity by its name in the entity namespace, so a fact whose
// whole text is that name is a different node (see Node).
func (n NamedEntity[K]) Hash(h hash.Hasher[K, string]) K {
	return h.Hash("entity:" + n.Value)
}

type Topic[K comparable] struct {
	ID K
	NodeAttributes

	Hasher hash.Hasher[K, string] `json:"-"`
}

func (t Topic[K]) Key() K {
	return t.Hash(t.Hasher)
}

func (t Topic[K]) GetValue() string {
	return t.Value
}

func (t Topic[K]) GetTimestamp() time.Time {
	return t.Timestamp
}

func (t *Topic[K]) GetAttributes() *NodeAttributes {
	return &t.NodeAttributes
}

// Hash keys the topic by its name in the topic namespace, so a fact whose whole
// text is that name is a different node (see Node).
func (t Topic[K]) Hash(h hash.Hasher[K, string]) K {
	return h.Hash("topic:" + t.Value)
}
