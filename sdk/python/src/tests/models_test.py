# MIT License

# Copyright (c) 2026 René-Jean Corneille

# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:

# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.

# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.

"""Parsing of the server's JSON envelope into the typed models — no I/O."""

from fraise_sdk.models import Hit, RecallResult


def test_from_json_defaults_to_no_warnings():
    """Omitting warnings yields an empty list, never None.

    This is the shape a clean response — or a pre-warnings server — produces,
    and a caller iterating ``result.warnings`` must not have to guard it.
    It is also the backward-compatible path for direct ``from_json`` callers
    that still pass only the results dict.
    """
    result = RecallResult.from_json({"count": 0, "hits": []})

    assert result.warnings == []


def test_from_json_carries_warnings_beside_the_hits():
    """Warnings pass through as a plain list of strings, and the hits they
    arrived beside parse exactly as they would without them.
    """
    result = RecallResult.from_json(
        {"count": 1, "hits": [{"value": "since the storm", "score": 1.0}]},
        warnings=["parse warning at column 15: ..."],
    )

    assert result.warnings == ["parse warning at column 15: ..."]
    assert result.count == 1
    assert [hit.value for hit in result] == ["since the storm"]


def test_hit_parses_provenance_and_contributions():
    """An explained hit keeps provenance distinct from ranking channels."""
    hit = Hit.from_json(
        {
            "value": "fact",
            "score": 0.75,
            "source": "session:abc",
            "contributions": [
                {
                    "source": "graph",
                    "score": 2,
                    "rank": 1,
                    "count": 2,
                    "via": "weather",
                    "degree": 3,
                }
            ],
        }
    )

    assert hit.source == "session:abc"
    assert hit.contributions is not None
    assert hit.contributions[0].channel == "graph"
    assert hit.contributions[0].via == "weather"
    assert hit.contributions[0].degree == 3


def test_old_hit_payload_remains_compatible():
    """A legacy hit without explain fields still parses with absent evidence."""
    hit = Hit.from_json({"value": "fact", "score": 1.0})

    assert hit.source is None
    assert hit.contributions is None


def test_recall_result_parses_versioned_explain_fields():
    """The explanation version travels with its query-level background rate."""
    result = RecallResult.from_json(
        {
            "count": 0,
            "hits": [],
            "background": 0.125,
            "explain_version": "unstable-1",
        }
    )

    assert result.background == 0.125
    assert result.explain_version == "unstable-1"
