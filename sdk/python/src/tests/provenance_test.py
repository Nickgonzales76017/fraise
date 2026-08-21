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
