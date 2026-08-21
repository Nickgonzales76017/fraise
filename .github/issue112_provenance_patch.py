from pathlib import Path
from textwrap import dedent


def rpl(path: str, old: str, new: str, count: int = 1) -> None:
    p = Path(path)
    text = p.read_text()
    if old not in text:
        raise SystemExit(f"missing patch anchor in {path}: {old[:100]!r}")
    p.write_text(text.replace(old, new, count))


def write(path: str, content: str) -> None:
    p = Path(path)
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(dedent(content).lstrip())


# First-class FQL source keyword.
rpl("internal/query/lexer/token.go", "\tDEPTH\n\n\t// param ref\n\tVEC", "\tDEPTH\n\tSOURCE\n\n\t// param ref\n\tVEC")
rpl("internal/query/lexer/token.go", '\tDEPTH:    "depth",\n\tLITERAL:', '\tDEPTH:    "depth",\n\tSOURCE:   "source",\n\tLITERAL:')
rpl("internal/query/lexer/token.go", '\t"depth":    DEPTH,\n\t"vec":      VEC,', '\t"depth":    DEPTH,\n\t"source":   SOURCE,\n\t"vec":      VEC,')
rpl("internal/query/lexer/token.go", "case RECALL, REMEMBER, FORGET, UPDATE, TOPIC, ENTITY, SINCE, UNTIL, TOP, DEPTH, VEC:", "case RECALL, REMEMBER, FORGET, UPDATE, TOPIC, ENTITY, SINCE, UNTIL, TOP, DEPTH, SOURCE, VEC:")

write("internal/query/parser/source.go", '''
// MIT License
package parser

import (
    "fmt"
    "strings"

    "github.com/RonsenbergVI/fraise/internal/query/lexer"
)

// SourceFieldNode is provenance metadata on a remembered fact. It is data, not
// fact identity. The field owns its quoted representation so spaces and
// punctuation cannot be re-tokenized when an AST is rendered back to FQL.
type SourceFieldNode struct {
    key   lexer.Token
    value string
}

func (n SourceFieldNode) String() string {
    return fmt.Sprintf("%s:'%s'", n.key.Literal, strings.ReplaceAll(n.value, "'", "''"))
}
func (n SourceFieldNode) Key() string         { return n.key.Literal }
func (n SourceFieldNode) Value() string       { return n.value }
func (n SourceFieldNode) Pos() lexer.Position { return n.key.Pos }
func (n SourceFieldNode) End() lexer.Position { return n.key.Pos }

// Source returns the one provenance reference carried by the remember command.
func (r RememberCommandNode[P]) Source() string {
    for _, a := range r.anchors {
        if f, ok := a.Field().(SourceFieldNode); ok {
            return f.Value()
        }
    }
    return ""
}
''')

# Accept source only on remember; preserve spelling and reject ambiguity.
rpl("internal/query/parser/parser.go", "\tvar anchors []AnchorFieldNode\n\n\tfor p.cur.Type != lexer.EOL {", "\tvar anchors []AnchorFieldNode\n\tsourceSeen := false\n\n\tfor p.cur.Type != lexer.EOL {")
rpl("internal/query/parser/parser.go", "\t\tcase lexer.VEC:\n\t\t\tvec, err := p.parseVecField()", '''\t\tcase lexer.SOURCE:
\t\t\tif sourceSeen {
\t\t\t\treturn nil, p.errf(p.cur.Pos, "duplicate source field")
\t\t\t}
\t\t\tsourceSeen = true
\t\t\tkey := p.cur
\t\t\tp.next()
\t\t\tif _, err := p.expect(lexer.COLON); err != nil {
\t\t\t\treturn nil, p.errf(p.cur.Pos, "Expected colon, but found %q", p.cur.Literal)
\t\t\t}
\t\t\ttok, err := p.expectValue()
\t\t\tif err != nil {
\t\t\t\treturn nil, err
\t\t\t}
\t\t\tanchors = append(anchors, AnchorFieldNode{field: SourceFieldNode{key: key, value: tok.Literal}})
\t\t\tr.anchors = anchors
\t\tcase lexer.VEC:
\t\t\tvec, err := p.parseVecField()''', 1)
rpl("internal/query/parser/ast.go", 'return fmt.Sprintf("%s%s%s:%s", c, n.token.Literal, n.field.Key(), n.field.Value())', 'return fmt.Sprintf("%s%s%s", c, n.token.Literal, n.field.String())')

# Promote parsed source into executable remember semantics.
rpl("internal/query/query.go", "\t\t\tTopics:   n.Topics(),\n\t\t}", "\t\t\tTopics:   n.Topics(),\n\t\t\tSource:   n.Source(),\n\t\t}")

# Explain-only provenance on the wire. Contributions nil is the existing mode bit.
rpl("internal/query/query.go", "func (h Hit[K, P]) MarshalJSON() ([]byte, error) {\n\tnode := *h.Node\n\n\treturn json.Marshal(struct {", '''func (h Hit[K, P]) MarshalJSON() ([]byte, error) {
\tnode := *h.Node
\tsource := ""
\tif h.Contributions != nil {
\t\tsource = node.GetAttributes().Source
\t}

\treturn json.Marshal(struct {''')
rpl("internal/query/query.go", '\t\tScore         P                    `json:"score"`\n\t\tContributions', '\t\tScore         P                    `json:"score"`\n\t\tSource        string               `json:"source,omitempty"`\n\t\tContributions')
rpl("internal/query/query.go", "\t\tScore:         h.Score,\n\t\tContributions:", "\t\tScore:         h.Score,\n\t\tSource:        source,\n\t\tContributions:")

# Persist metadata without changing Fact.Hash identity. Cache identity changes
# only when a non-empty source changes persisted write semantics.
rpl("internal/graph/node.go", "type NodeAttributes struct {\n\tValue     string\n\tTimestamp time.Time\n}", "type NodeAttributes struct {\n\tValue     string\n\tTimestamp time.Time\n\tSource    string\n}")
rpl("internal/query/remember.go", "type Remember[K comparable, P float32 | float64] struct {\n\tValue    string\n\tEntities []string\n\tTopics   []string\n\tVector", "type Remember[K comparable, P float32 | float64] struct {\n\tValue    string\n\tEntities []string\n\tTopics   []string\n\tSource   string\n\tVector")
rpl("internal/query/remember.go", '\tb.WriteString("|to=")\n\tb.WriteString(strings.Join(r.Topics, "\\x00"))\n\tb.WriteString("|vec=")', '\tb.WriteString("|to=")\n\tb.WriteString(strings.Join(r.Topics, "\\x00"))\n\tif r.Source != "" {\n\t\tb.WriteString("|src=")\n\t\tb.WriteString(r.Source)\n\t}\n\tb.WriteString("|vec=")')
rpl("internal/query/stream.go", "\t\t\t\tValue:     remember.Value,\n\t\t\t\tTimestamp: time.Now(),", "\t\t\t\tValue:     remember.Value,\n\t\t\t\tTimestamp: time.Now(),\n\t\t\t\tSource:    remember.Source,")

# Python construction path.
rpl("sdk/python/src/fraise_sdk/query.py", "    entities: Sequence[str] | None = None,\n    with_vector: bool = False,", "    entities: Sequence[str] | None = None,\n    source: str | None = None,\n    with_vector: bool = False,")
rpl("sdk/python/src/fraise_sdk/query.py", '    parts += _clauses("entity", entities)\n    if with_vector:', '    parts += _clauses("entity", entities)\n    if source is not None:\n        parts.append(f"source:{_quote_value(source)}")\n    if with_vector:')
rpl("sdk/python/src/fraise_sdk/client.py", "        entities: Sequence[str] | None = None,\n        vector: Sequence[float] | None = None,", "        entities: Sequence[str] | None = None,\n        source: str | None = None,\n        vector: Sequence[float] | None = None,", 1)
rpl("sdk/python/src/fraise_sdk/client.py", "            topics=topics,\n            entities=entities,\n            with_vector=resolved is not None,", "            topics=topics,\n            entities=entities,\n            source=source,\n            with_vector=resolved is not None,", 1)

# Python typed explain model remains backward-compatible for old /q payloads.
rpl("sdk/python/src/fraise_sdk/models.py", "@dataclass(frozen=True)\nclass Hit:", '''@dataclass(frozen=True)
class Contribution:
    # One deterministic scoring observation in an explained recall.
    source: str
    score: float
    rank: int
    count: int
    via: str | None = None
    degree: int | None = None

    @classmethod
    def from_json(cls, data: dict) -> "Contribution":
        return cls(
            source=data["source"],
            score=float(data["score"]),
            rank=int(data.get("rank", 0)),
            count=int(data.get("count", 0)),
            via=data.get("via"),
            degree=int(data["degree"]) if data.get("degree") is not None else None,
        )


@dataclass(frozen=True)
class Hit:''')
rpl("sdk/python/src/fraise_sdk/models.py", "    timestamp: str | None = None\n\n    @classmethod", "    timestamp: str | None = None\n    source: str | None = None\n    contributions: list[Contribution] = field(default_factory=list)\n\n    @classmethod")
rpl("sdk/python/src/fraise_sdk/models.py", '            timestamp=data.get("timestamp"),\n        )', '            timestamp=data.get("timestamp"),\n            source=data.get("source"),\n            contributions=[Contribution.from_json(c) for c in data.get("contributions") or []],\n        )', 1)
rpl("sdk/python/src/fraise_sdk/models.py", "    warnings: list[str] = field(default_factory=list)\n\n    @classmethod", "    warnings: list[str] = field(default_factory=list)\n    background: float | None = None\n\n    @classmethod")
rpl("sdk/python/src/fraise_sdk/models.py", "            hits=hits,\n            warnings=list(warnings or []),\n        )", '            hits=hits,\n            warnings=list(warnings or []),\n            background=float(results["background"]) if results.get("background") is not None else None,\n        )')

write("internal/query/parser/source_test.go", '''
package parser

import "testing"

func TestRememberSourceRoundTrip(t *testing.T) {
    cmd, _, err := Parse[string, float32]("remember@2 'fact' source:'Tool Call / Session 17'")
    if err != nil { t.Fatalf("Parse() error = %v", err) }
    remember := cmd.(*RememberCommandNode[float32])
    if got, want := remember.Source(), "Tool Call / Session 17"; got != want { t.Fatalf("Source() = %q, want %q", got, want) }
    if got, want := remember.String(), "remember@2 'fact' source:'Tool Call / Session 17'"; got != want { t.Fatalf("String() = %q, want %q", got, want) }
}

func TestRememberRejectsDuplicateSource(t *testing.T) {
    _, _, err := Parse[string, float32]("remember 'fact' source:'one' source:'two'")
    if err == nil { t.Fatal("duplicate source parsed successfully") }
}

func TestRecallDoesNotAcceptSourceClause(t *testing.T) {
    _, _, err := Parse[string, float32]("recall fact source:'one'")
    if err == nil { t.Fatal("source clause unexpectedly accepted on recall") }
}
''')

write("internal/query/provenance_test.go", '''
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
    if a.Hash(&fakeHasher{}) == b.Hash(&fakeHasher{}) { t.Fatal("different provenance references shared a plan-cache key") }
    if legacy := base.Hash(&fakeHasher{}); strings.Contains(legacy, "|src=") { t.Fatalf("empty provenance changed legacy cache identity: %q", legacy) }
}

func TestHitSourceAppearsOnlyInExplainMode(t *testing.T) {
    var node graph.Node[string] = graph.Fact[string]{NodeAttributes: graph.NodeAttributes{Value: "fact", Timestamp: time.Unix(1, 0).UTC(), Source: "session:abc"}}
    plain, err := json.Marshal(Hit[string, float32]{Node: &node, Score: 1})
    if err != nil { t.Fatal(err) }
    if strings.Contains(string(plain), `"source"`) { t.Fatalf("plain recall leaked provenance: %s", plain) }
    explained, err := json.Marshal(Hit[string, float32]{Node: &node, Score: 1, Contributions: []HitContribution[float32]{{Source: "text", Score: 1, Rank: 0, Count: 1}}})
    if err != nil { t.Fatal(err) }
    if !strings.Contains(string(explained), `"source":"session:abc"`) { t.Fatalf("explained recall omitted provenance: %s", explained) }
}
''')

write("sdk/python/src/tests/provenance_test.py", '''
from fraise_sdk.models import Hit, RecallResult
from fraise_sdk.query import build_remember


def test_build_remember_quotes_source_reference():
    assert build_remember("fact", graph=2, source="Tool Call / Nick's session") == "remember@2 'fact' source:'Tool Call / Nick''s session'"


def test_hit_parses_provenance_and_contributions():
    hit = Hit.from_json({"value": "fact", "score": 0.75, "source": "session:abc", "contributions": [{"source": "graph", "score": 2, "rank": 1, "count": 2, "via": "weather", "degree": 3}]})
    assert hit.source == "session:abc"
    assert hit.contributions[0].via == "weather"
    assert hit.contributions[0].degree == 3


def test_old_hit_payload_remains_compatible():
    hit = Hit.from_json({"value": "fact", "score": 1.0})
    assert hit.source is None
    assert hit.contributions == []


def test_recall_result_parses_explain_background():
    assert RecallResult.from_json({"count": 0, "hits": [], "background": 0.125}).background == 0.125
''')
