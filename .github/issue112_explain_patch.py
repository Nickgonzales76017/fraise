from pathlib import Path
from textwrap import dedent


def rpl(path: str, old: str, new: str, count: int = 1) -> None:
    p = Path(path)
    text = p.read_text()
    if old not in text:
        raise SystemExit(f"missing patch anchor in {path}: {old[:120]!r}")
    p.write_text(text.replace(old, new, count))


def append_once(path: str, marker: str, content: str) -> None:
    p = Path(path)
    text = p.read_text()
    if marker not in text:
        p.write_text(text.rstrip() + "\n\n" + dedent(content).lstrip())


def write(path: str, content: str) -> None:
    p = Path(path)
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(dedent(content).lstrip())


# Typed explain mirrors recall but selects the existing side-effect-free explain endpoint.
client = Path("sdk/python/src/fraise_sdk/client.py")
text = client.read_text()
if "    def explain(\n" not in text:
    anchor = "\n    # -- embedding ---------------------------------------------------------\n"
    method = r'''

    def explain(
        self,
        *keywords: str,
        graph: int = 0,
        query: str | None = None,
        topics: Sequence[str] | None = None,
        entities: Sequence[str] | None = None,
        top: int | None = None,
        depth: int | None = None,
        vector: Sequence[float] | None = None,
        embed: bool | None = None,
        timeout: float | None = None,
    ) -> RecallResult:
        """Run a recall through ``/api/v1/explain`` and return typed evidence.

        The ranking plan is identical to :meth:`recall`; the server attaches
        provenance and deterministic contribution records only to this request.
        ``score`` remains a ranking score, not a calibrated probability.
        """
        embed_text = query if query is not None else " ".join(keywords)
        resolved = self._resolve_vector(vector, embed_text, embed)
        text = _query.build_recall(
            keywords=list(keywords),
            graph=graph,
            query=query,
            topics=topics,
            entities=entities,
            top=top,
            depth=depth,
            with_vector=resolved is not None,
        )
        parameters = {_query.VECTOR_PARAM: resolved} if resolved is not None else None
        body = self.query(
            text,
            parameters=parameters,
            timeout=timeout,
            _endpoint="explain",
        )
        results = body.get("results") or {}
        return RecallResult.from_json(results, warnings=body.get("warnings"))
'''
    if anchor not in text:
        raise SystemExit("embedding section anchor not found")
    text = text.replace(anchor, method + anchor, 1)

# Keep query() backward-compatible while letting typed helpers select a trusted endpoint.
old_sig = '''    def query(
        self,
        text: str,
        *,
        parameters: dict[str, list[float]] | None = None,
        timeout: float | None = None,
    ) -> dict:
'''
new_sig = '''    def query(
        self,
        text: str,
        *,
        parameters: dict[str, list[float]] | None = None,
        timeout: float | None = None,
        _endpoint: str = "q",
    ) -> dict:
'''
if old_sig in text:
    text = text.replace(old_sig, new_sig, 1)
elif '_endpoint: str = "q"' not in text:
    raise SystemExit("query signature anchor not found")
text = text.replace('f"{self.base_url}/api/v1/q",', 'f"{self.base_url}/api/v1/{_endpoint}",', 1)
client.write_text(text)

# Mock-level SDK proof.
rpl(
    "sdk/python/src/tests/client_test.py",
    'QUERY_URL = f"{DEFAULT_BASE_URL}/api/v1/q"\n',
    'QUERY_URL = f"{DEFAULT_BASE_URL}/api/v1/q"\nEXPLAIN_URL = f"{DEFAULT_BASE_URL}/api/v1/explain"\n',
)
append_once(
    "sdk/python/src/tests/client_test.py",
    "def test_typed_explain_posts_to_explain_endpoint",
    r'''

def test_typed_explain_posts_to_explain_endpoint(session):
    _respond(
        session,
        {
            "results": {
                "count": 1,
                "background": 0.125,
                "hits": [
                    {
                        "value": "deploys require two approvals",
                        "score": 0.75,
                        "source": "github:policy/17",
                        "contributions": [
                            {"source": "text", "score": 1, "rank": 0, "count": 1}
                        ],
                    }
                ],
            }
        },
    )
    result = FraiseClient().explain("deploys", "approvals", graph=2)

    session.post.assert_called_once_with(
        EXPLAIN_URL,
        json={"query": "recall@2 deploys approvals"},
        timeout=DEFAULT_TIMEOUT_SECONDS,
    )
    assert result.hits[0].source == "github:policy/17"
    assert result.hits[0].contributions[0].source == "text"
    assert result.background == 0.125
''',
)

# Real server round-trip: persisted provenance is explain-only.
append_once(
    "tests/e2e/explain_test.py",
    "def test_explain_returns_remembered_provenance_but_plain_query_stays_lean",
    r'''

def test_explain_returns_remembered_provenance_but_plain_query_stays_lean(query, explain):
    phrase = "traceability probe remembers its origin"
    source = "agent-session:proof-17/tool-call:4"
    status, body = query(
        f"remember@2 '{phrase}' topic:traceability source:'{source}'"
    )
    assert status == 200, body.get("error")

    status, body = explain("recall@2 traceability")
    assert status == 200, body.get("error")
    hit = next(h for h in body["results"]["hits"] if h["value"] == phrase)
    assert hit["source"] == source
    assert hit["contributions"], "explained provenance still carries scoring evidence"

    status, body = query("recall@2 traceability")
    assert status == 200, body.get("error")
    plain = next(h for h in body["results"]["hits"] if h["value"] == phrase)
    assert "source" not in plain
    assert "contributions" not in plain
''',
)

# Replace the stale hop-era explanation documentation with current semantics.
http = Path("docs/http-api.md")
http_text = http.read_text()
start = http_text.find("## `POST /api/v1/explain` — explained recall")
end = http_text.find("\n## `GET /api/v1/stats`", start)
if start == -1 or end == -1:
    raise SystemExit("explain docs section not found")
section = dedent(r'''
## `POST /api/v1/explain` — explained recall

The same request body and ranking pipeline as `/q`, for recalls only. Explain is
request-local and is selected after planning, so asking for evidence does not
change the plan-cache identity. Each hit may add two forms of evidence:

- `source`: the provenance reference stored by `remember ... source:'...'`;
- `contributions`: deterministic observations from `text`, `vector`, or `graph`
  that produced the ranking.

The query-level `background` field is included when graph surplus contributes.
A graph contribution can include `via` (the funding topic/entity), `degree`, and
`count`; the current wire format does **not** use the old hop field.

```json
{
  "results": {
    "count": 1,
    "background": 0.125,
    "hits": [
      {
        "value": "deploys require two approvals",
        "timestamp": "2026-08-21T01:40:00Z",
        "score": 0.75,
        "source": "github:policy/17",
        "contributions": [
          {"source": "text", "score": 1, "rank": 0, "count": 1},
          {"source": "graph", "score": 2, "rank": 1, "count": 2,
           "via": "deploy-policy", "degree": 3}
        ]
      }
    ]
  }
}
```

`score` is a deterministic ranking score, **not a calibrated probability**.
Consumers that need confidence should inspect the provenance and contribution
structure, compare competing hits, and apply their own decision threshold.
Ordinary `/q` deliberately omits `source`, `contributions`, and `background` so
routine agent recalls keep their small response shape.

A `remember` on this endpoint is rejected with 400: explanation is read-only.
''').strip() + "\n"
http.write_text(http_text[:start] + section + http_text[end:])

write(
    "docs/memory-traceability.md",
    r'''
# Memory traceability

Fraise can keep a bounded provenance reference with a memory and expose it only
when an agent asks why that memory ranked. Provenance is metadata, not ownership
or correctness: the reference says where the memory came from; it does not make
the source trustworthy.

```python
from fraise_sdk import FraiseClient

client = FraiseClient()
client.remember(
    "deploys require two approvals",
    graph=2,
    topics=["deploy-policy"],
    source="github:policy/17",
)

# Routine recall stays lean.
plain = client.recall("deploys", "approvals", graph=2)
assert plain.hits[0].source is None
assert plain.hits[0].contributions == []

# Ask for the evidence only at the decision boundary.
explained = client.explain("deploys", "approvals", graph=2)
hit = explained.hits[0]
print(hit.source)          # github:policy/17
print(hit.score)           # ranking score, not a probability
for observation in hit.contributions:
    print(observation.source, observation.score, observation.via)
```

A useful agent policy is to use `recall()` during normal context assembly and
call `explain()` only before a consequential action, when a surprising memory
wins, or when competing memories disagree. That keeps token cost low while
retaining a deterministic path from ranked memory back to stored provenance and
ranking evidence.
''',
)
