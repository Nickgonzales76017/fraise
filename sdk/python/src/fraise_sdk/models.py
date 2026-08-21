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

"""Typed views over the server's JSON responses."""

from __future__ import annotations

from collections.abc import Iterator, Sequence
from dataclasses import dataclass, field


@dataclass(frozen=True)
class HitContribution:
    """One channel's contribution to a hit's score, from an explain payload.

    ``channel`` names where the evidence came from — ``"text"``, ``"vector"``
    or ``"graph"``. It is read from the wire's ``source`` key, but is not
    called ``source`` here: an explained hit now carries a ``source`` of its
    own meaning the fact's *provenance*, and two different things spelled the
    same in one payload is how a caller ends up reporting the retrieval
    channel as the origin of a memory.

    A ``graph`` contribution additionally carries ``via`` (the anchor that
    funded it, by value), its ``degree``, and the ``count`` of seeds that
    reached it; ``text`` and ``vector`` contributions carry raw mass and list
    position only.
    """

    channel: str
    score: float
    rank: int = 0
    via: str | None = None
    degree: int = 0
    count: int = 0

    @classmethod
    def from_json(cls, data: dict) -> HitContribution:  # noqa: D102
        return cls(
            channel=data["source"],
            score=float(data["score"]),
            rank=int(data.get("rank", 0)),
            via=data.get("via"),
            degree=int(data.get("degree", 0)),
            count=int(data.get("count", 0)),
        )


@dataclass(frozen=True)
class Hit:
    """One recalled fact, how strongly it matched, and — under explain — why.

    ``source`` is the fact's provenance: the reference recorded with it when it
    was remembered. ``contributions`` is the per-channel breakdown of its
    score. Both are populated only by an explained recall, and both default to
    "absent" rather than to an empty value: a server that predates them, or a
    fact stored without an origin, parses to ``None`` and never to ``""``,
    so "no provenance recorded" stays distinguishable from "provenance is the
    empty string".
    """

    value: str
    score: float
    timestamp: str | None = None
    source: str | None = None
    contributions: tuple[HitContribution, ...] | None = None

    @classmethod
    def from_json(cls, data: dict) -> Hit:  # noqa: D102
        contributions = data.get("contributions")
        return cls(
            value=data["value"],
            score=float(data["score"]),
            timestamp=data.get("timestamp"),
            source=data.get("source"),
            contributions=(
                tuple(HitContribution.from_json(c) for c in contributions)
                if contributions is not None
                else None
            ),
        )


@dataclass(frozen=True)
class RecallResult:
    """The result set of a ``recall``: how many facts matched and, in ranked order, what they were.

    ``warnings`` carries any parse warnings the server attached: the query ran
    and the hits are valid, but it was one typo away from meaning something
    else (e.g. a leading term that spells a grammar keyword). Empty on the
    common, unambiguous path.

    ``explain_version`` and ``background`` appear only for an explained recall.
    ``explain_version`` names the shape of that payload and currently carries an
    ``unstable-`` prefix: the explain envelope is a debugging surface, not a
    stability commitment, so a caller that reads the breakdown should check it
    rather than assume the fields it saw last release. ``background`` is the
    query's background rate, the one query-level term a caller needs to
    recompute a hit's score from its own contributions.
    """

    count: int
    hits: list[Hit]
    warnings: list[str] = field(default_factory=list)
    background: float = 0.0
    explain_version: str | None = None

    @classmethod
    def from_json(
        cls, results: dict, warnings: Sequence[str] | None = None
    ) -> RecallResult:
        """Parse the server's ``results`` object, with any response warnings.

        Args:
            results: the ``results`` member of the response body.
            warnings: the response's ``warnings`` list, which sits beside
                ``results`` rather than inside it — the caller holding the
                whole body passes it through here. ``None`` (a clean response,
                or a pre-warnings server) parses to an empty list.

        Returns:
            The typed result, warnings included.
        """
        hits = [Hit.from_json(h) for h in results.get("hits") or []]
        # Prefer the server-reported count, falling back to the hit count so the
        # two never disagree if the field is ever omitted.
        return cls(
            count=results.get("count", len(hits)),
            hits=hits,
            warnings=list(warnings or []),
            # Both explain fields are omitted by an ordinary recall, so their
            # absence is the mode rather than an error: a plain result reads
            # back as background 0.0 and no version, exactly as it did before
            # explain existed.
            background=float(results.get("background", 0.0)),
            explain_version=results.get("explain_version"),
        )

    def __bool__(self) -> bool:
        return bool(self.hits)

    def __iter__(self) -> Iterator[Hit]:
        return iter(self.hits)

    def __len__(self) -> int:
        return len(self.hits)
