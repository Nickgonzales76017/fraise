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

package query_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/RonsenbergVI/fraise/internal/config"
	"github.com/RonsenbergVI/fraise/internal/graph"
	"github.com/RonsenbergVI/fraise/internal/query"
)

// These tests exercise Parse, which turns a query string into the concrete
// *Remember/*Recall command it dispatches to.

// TestParseRemember checks that a remember query is dispatched to a
// *query.Remember carrying the phrase value and the graph selector,
// and reporting as a write.
func TestParseRemember(t *testing.T) {
	q := "remember@1 'anne loves the color orange' topic:color topic:preference entity:anne"

	got, _, err := query.Parse[string, float32](q, nil, config.New())
	if err != nil {
		t.Fatalf("Parse(%q) returned unexpected error: %v", q, err)
	}

	r, ok := got.(*query.Remember[string, float32])
	if !ok {
		t.Fatalf("Parse(%q) returned %T, want *query.Remember", q, got)
	}

	// The phrase value is the raw text: the quotes are query syntax, not data.
	if want := "anne loves the color orange"; r.Value != want {
		t.Errorf("Value = %q, want %q", r.Value, want)
	}
	if want := []string{"anne"}; !reflect.DeepEqual(r.Entities, want) {
		t.Errorf("Entities = %v, want %v", r.Entities, want)
	}
	if want := []string{"color", "preference"}; !reflect.DeepEqual(r.Topics, want) {
		t.Errorf("Topics = %v, want %v", r.Topics, want)
	}
	if id := r.GetGraphID(); id != 1 {
		t.Errorf("GetGraphID() = %d, want 1", id)
	}
	if !r.IsWrite() {
		t.Error("Remember.IsWrite() = false, want true")
	}
}

// TestParseRecall checks that a recall query is dispatched to a *query.Recall
// carrying its terms, entities, topics and graph selector, and reporting as a
// read.
func TestParseRecall(t *testing.T) {
	q := "recall@2 anna bob entity:alice topic:job"

	got, _, err := query.Parse[string, float32](q, nil, config.New())
	if err != nil {
		t.Fatalf("Parse(%q) returned unexpected error: %v", q, err)
	}

	r, ok := got.(*query.Recall[string, float32])
	if !ok {
		t.Fatalf("Parse(%q) returned %T, want *query.Recall", q, got)
	}

	if want := []string{"anna", "bob"}; !reflect.DeepEqual(r.Keywords, want) {
		t.Errorf("Keywords = %v, want %v", r.Keywords, want)
	}
	if want := []string{"alice"}; !reflect.DeepEqual(r.Entities, want) {
		t.Errorf("Entities = %v, want %v", r.Entities, want)
	}
	if want := []string{"job"}; !reflect.DeepEqual(r.Topics, want) {
		t.Errorf("Topics = %v, want %v", r.Topics, want)
	}
	if id := r.GetGraphID(); id != 2 {
		t.Errorf("GetGraphID() = %d, want 2", id)
	}
	if r.IsWrite() {
		t.Error("Recall.IsWrite() = true, want false")
	}
}

// TestParseErrors checks that malformed or empty queries are rejected with
// ErrParsingFailed and a nil query.
func TestParseErrors(t *testing.T) {
	queries := []string{
		"",                 // empty input
		"recall",           // a recall must carry at least one term
		"recall topic:job", // fields may only follow a term
		"forget anna",      // unknown command keyword
	}

	for _, q := range queries {
		t.Run(q, func(t *testing.T) {
			got, _, err := query.Parse[string, float32](q, nil, config.New())
			if !errors.Is(err, query.ErrParsingFailed) {
				t.Errorf("Parse(%q) err = %v, want it to wrap ErrParsingFailed", q, err)
			}
			if got != nil {
				t.Errorf("Parse(%q) = %v, want nil query", q, got)
			}
		})
	}
}

// TestParseRecallBindsVector checks that a vec:$v placeholder is bound from the
// supplied parameters into the resulting *query.Recall, and that the parser
// itself never touches the vector data.
func TestParseRecallBindsVector(t *testing.T) {
	q := "recall@0 amelia entity:amelia topic:preferences vec:$v"
	params := map[string][]float32{"v": {0.1, 0.2, 0.3}}

	got, _, err := query.Parse[string, float32](q, params, config.New())
	if err != nil {
		t.Fatalf("Parse(%q) returned unexpected error: %v", q, err)
	}

	r, ok := got.(*query.Recall[string, float32])
	if !ok {
		t.Fatalf("Parse(%q) returned %T, want *query.Recall", q, got)
	}

	if want := []float32{0.1, 0.2, 0.3}; !reflect.DeepEqual(r.Vector.Data, want) {
		t.Errorf("Vector.Data = %v, want %v", r.Vector.Data, want)
	}
}

// TestParseRecallMissingParameter checks that referencing a placeholder with no
// matching parameter is rejected with ErrMissingParameter.
func TestParseRecallMissingParameter(t *testing.T) {
	q := "recall@0 amelia vec:$v"

	got, _, err := query.Parse[string, float32](q, nil, config.New())
	if !errors.Is(err, query.ErrMissingParameter) {
		t.Errorf("Parse(%q) err = %v, want ErrMissingParameter", q, err)
	}
	if got != nil {
		t.Errorf("Parse(%q) = %v, want nil query", q, got)
	}
}

// TestParseRejectsOverLimits checks that the configured ceilings are enforced at
// parse time: a recall over the top/depth ceiling, or a bound vector longer than
// the dimension ceiling, is rejected with ErrLimitExceeded and no query.
func TestParseRejectsOverLimits(t *testing.T) {
	cfg := config.New()
	cfg.DB.MaxTop = 10
	cfg.DB.MaxDepth = 2
	cfg.DB.MaxVectorDimension = 3
	cfg.DB.MaxSourceLength = 10

	cases := []struct {
		name   string
		query  string
		params map[string][]float32
	}{
		{"top over ceiling", "recall@0 anna top:99", nil},
		{"depth over ceiling", "recall@0 anna depth:9", nil},
		{"recall vector too long", "recall@0 anna vec:$v", map[string][]float32{"v": {1, 2, 3, 4}}},
		{"remember vector too long", "remember@0 'a fact' vec:$v", map[string][]float32{"v": {1, 2, 3, 4}}},
		{"remember source too long", "remember@0 'a fact' source:'" + strings.Repeat("x", 11) + "'", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := query.Parse[string, float32](tc.query, tc.params, cfg)
			if !errors.Is(err, query.ErrLimitExceeded) {
				t.Errorf("Parse(%q) err = %v, want ErrLimitExceeded", tc.query, err)
			}
			if got != nil {
				t.Errorf("Parse(%q) = %v, want nil query", tc.query, got)
			}
		})
	}

	// A request within every ceiling still parses cleanly.
	if _, _, err := query.Parse[string, float32]("recall@0 anna top:5 depth:1", nil, cfg); err != nil {
		t.Errorf("within-limit recall returned error: %v", err)
	}
	if _, _, err := query.Parse[string, float32]("remember@0 'a fact' source:'doc://ok'", nil, cfg); err != nil {
		t.Errorf("within-limit remember returned error: %v", err)
	}
}

// marshalHit builds a Hit over a fact with a fixed timestamp and returns its
// JSON. The fact needs no hasher: marshalling reads only value and timestamp.
func marshalHit(t *testing.T, contributions []query.HitContribution[float32]) string {
	t.Helper()
	var node graph.Node[string] = graph.Fact[string]{NodeAttributes: graph.NodeAttributes{
		Value:     "the parrot is turquoise",
		Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}}
	h := query.Hit[string, float32]{Node: &node, Score: 0.5, Contributions: contributions}
	out, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("Marshal(Hit) = %v, want nil", err)
	}
	return string(out)
}

// TestHitMarshalOmitsContributionsByDefault pins the ordinary wire shape: a
// hit without contributions serializes exactly as it did before explain
// existed — no "contributions" key, so /q responses did not change byte for
// byte when the explain endpoint landed.
func TestHitMarshalOmitsContributionsByDefault(t *testing.T) {
	got := marshalHit(t, nil)
	want := `{"value":"the parrot is turquoise","timestamp":"2026-01-02T03:04:05Z","score":0.5}`
	if got != want {
		t.Errorf("Marshal(Hit) = %s, want %s", got, want)
	}
}

// TestHitMarshalSerializesContributions pins the explain wire shape: each
// contribution carries its source by name — the payload documents ranking to
// clients that never see the Go constants — with its raw score and rank, and,
// for a graph entry, the funding anchor's value under via, its degree, and
// its funding-seed count. via and degree are omitted for seed sources, where
// they mean nothing.
func TestHitMarshalSerializesContributions(t *testing.T) {
	got := marshalHit(t, []query.HitContribution[float32]{
		{Source: "text", Score: 1, Rank: 0, Count: 1},
		{Source: "vector", Score: 0.5, Rank: 1, Count: 1},
		{Source: "graph", Score: 2, Via: "weather", Degree: 3, Count: 2},
	})
	want := `{"value":"the parrot is turquoise","timestamp":"2026-01-02T03:04:05Z","score":0.5,` +
		`"contributions":[` +
		`{"source":"text","score":1,"rank":0,"count":1},` +
		`{"source":"vector","score":0.5,"rank":1,"count":1},` +
		`{"source":"graph","score":2,"rank":0,"via":"weather","degree":3,"count":2}]}`
	if got != want {
		t.Errorf("Marshal(Hit) = %s, want %s", got, want)
	}
}

// TestParseRememberBindsSource checks the provenance clause reaches the
// executable query: the parser records it, Parse copies it onto the Remember,
// and the fact's other fields are untouched by its presence.
func TestParseRememberBindsSource(t *testing.T) {
	const q = "remember@1 'acme moved to annual billing' topic:billing source:'doc://contracts/Acme-2026.pdf#p4'"

	got, _, err := query.Parse[string, float32](q, nil, config.New())
	if err != nil {
		t.Fatalf("Parse(%q) err = %v, want nil", q, err)
	}
	r, ok := got.(*query.Remember[string, float32])
	if !ok {
		t.Fatalf("Parse(%q) = %T, want *query.Remember", q, got)
	}

	if want := "doc://contracts/Acme-2026.pdf#p4"; r.Source != want {
		t.Errorf("Source = %q, want %q", r.Source, want)
	}
	if want := "acme moved to annual billing"; r.Value != want {
		t.Errorf("Value = %q, want %q", r.Value, want)
	}
	if want := []string{"billing"}; !reflect.DeepEqual(r.Topics, want) {
		t.Errorf("Topics = %v, want %v", r.Topics, want)
	}

	// A remember without the clause carries no provenance — not an empty one
	// invented on the way through.
	bare, _, err := query.Parse[string, float32]("remember@1 'acme moved to annual billing'", nil, config.New())
	if err != nil {
		t.Fatalf("Parse of unsourced remember err = %v, want nil", err)
	}
	if src := bare.(*query.Remember[string, float32]).Source; src != "" {
		t.Errorf("unsourced remember got Source = %q, want empty", src)
	}
}

// TestSourceCeilingBoundsProvenanceToAReference is the privacy fence stated as
// a limit rather than as advice. A source is meant to be a reference to an
// origin; nothing in the grammar stops a caller pasting the origin itself —
// a whole tool payload, with whatever credential it happened to carry —
// except this ceiling, which rejects it at parse time, before any of it is
// written to a graph, a log line or an explain payload.
func TestSourceCeilingBoundsProvenanceToAReference(t *testing.T) {
	cfg := config.New()

	// A realistic tool payload: far longer than a reference, and carrying a
	// secret the caller never meant to persist.
	payload := `{"tool":"vault.read","token":"` + strings.Repeat("A", cfg.DB.MaxSourceLength) + `-CANARY"}`
	q := "remember@0 'the deploy key rotated' source:'" + payload + "'"

	got, _, err := query.Parse[string, float32](q, nil, cfg)
	if !errors.Is(err, query.ErrLimitExceeded) {
		t.Fatalf("Parse of an origin-sized source err = %v, want ErrLimitExceeded", err)
	}
	if got != nil {
		t.Error("Parse returned a query for a rejected source; nothing must reach the engine")
	}
	if strings.Contains(err.Error(), "CANARY") {
		t.Error("the rejection message quoted the payload back: the error is a second leak channel")
	}

	// The reference form of the same origin is exactly what should be stored,
	// and it is well inside the ceiling.
	ref := "tool://vault.read/call-17"
	if len(ref) > cfg.DB.MaxSourceLength {
		t.Fatalf("test premise broken: reference %q is over the ceiling", ref)
	}
	if _, _, err := query.Parse[string, float32]("remember@0 'the deploy key rotated' source:'"+ref+"'", nil, cfg); err != nil {
		t.Errorf("Parse of a reference-sized source err = %v, want nil", err)
	}
}

// TestHitMarshalOmitsSourceByDefault pins the compatibility half of
// provenance: a hit with no source serializes exactly as it did before source
// existed. omitempty is the mechanism, and this is the test that stops it
// being dropped — an unsourced fact must not start emitting "source":"" into
// every /q response.
func TestHitMarshalOmitsSourceByDefault(t *testing.T) {
	got := marshalHit(t, nil)
	if strings.Contains(got, "source") {
		t.Errorf("Hit JSON = %s, want no source key", got)
	}
	want := `{"value":"the parrot is turquoise","timestamp":"2026-01-02T03:04:05Z","score":0.5}`
	if got != want {
		t.Errorf("Hit JSON = %s, want %s", got, want)
	}
}

// TestHitMarshalSerializesSource is the other side: when the read path did
// attach provenance, it reaches the client under "source".
func TestHitMarshalSerializesSource(t *testing.T) {
	var node graph.Node[string] = graph.Fact[string]{
		NodeAttributes: graph.NodeAttributes{
			Value:     "the parrot is turquoise",
			Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		},
		Source: "doc://aviary-log#12",
	}
	h := query.Hit[string, float32]{Node: &node, Score: 0.5, Source: "doc://aviary-log#12"}

	out, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("Marshal(Hit) = %v, want nil", err)
	}
	want := `{"value":"the parrot is turquoise","timestamp":"2026-01-02T03:04:05Z","score":0.5,"source":"doc://aviary-log#12"}`
	if string(out) != want {
		t.Errorf("Hit JSON = %s, want %s", out, want)
	}
}
