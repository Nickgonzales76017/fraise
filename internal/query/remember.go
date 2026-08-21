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
	"strconv"
	"strings"

	"github.com/RonsenbergVI/fraise/internal/config"
	"github.com/RonsenbergVI/fraise/internal/containers"
	"github.com/RonsenbergVI/fraise/internal/hash"
)

type Remember[K comparable, P float32 | float64] struct {
	Value    string
	Entities []string
	Topics   []string
	Source   string
	Vector   containers.Vector[K, P]

	context QueryContext
}

func (r *Remember[K, P]) Plan(config *config.ConfigSet) (*Stream[K, P], error) {
	return NewStream(r), nil
}

func (r Remember[K, P]) GetGraphID() uint8 {
	return r.context.GraphID
}

func (r *Remember[K, P]) SetGraphID(id uint8) {
	r.context.GraphID = id
}

// Hash keys the query for the plan cache. Like Recall it must fold in the graph
// selector and every field that changes what gets written — including the bound
// vector: hashing only Value would make `remember@3 'x' topic:a` and
// `remember@5 'x' topic:b` collide, so the second would reuse the first's plan
// and write to the wrong graph.
//
// Source is one of those fields. It does not change *which* fact is written —
// Fact.Hash is the fact's text alone — but it changes what is written with it,
// so leaving it out would let two remembers of the same sentence from two
// origins share one cached plan: the second call would silently persist the
// first call's provenance, and the memory would name the wrong origin with no
// error anywhere. That is the precise failure traceability exists to prevent,
// so it is fenced by a regression test.
func (r Remember[K, P]) Hash(h hash.Hasher[K, string]) K {
	var b strings.Builder
	b.WriteString("g=")
	b.WriteString(strconv.Itoa(int(r.context.GraphID)))
	b.WriteString("|v=")
	b.WriteString(r.Value)
	b.WriteString("|en=")
	b.WriteString(strings.Join(r.Entities, "\x00"))
	b.WriteString("|to=")
	b.WriteString(strings.Join(r.Topics, "\x00"))
	if r.Source != "" {
		b.WriteString("|src=")
		b.WriteString(r.Source)
	}
	b.WriteString("|vec=")
	b.WriteString(r.Vector.Hash(h))
	return h.Hash(b.String())
}

func (r Remember[K, P]) IsWrite() bool {
	return true
}
