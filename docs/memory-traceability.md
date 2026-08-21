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
assert plain.hits[0].contributions is None

# Ask for the evidence only at the decision boundary.
explained = client.recall("deploys", "approvals", graph=2, explain=True)
assert explained.explain_version == "unstable-1"
hit = explained.hits[0]
print(hit.source)          # github:policy/17
print(hit.score)           # ranking score, not a probability
for observation in hit.contributions:
    print(observation.channel, observation.score, observation.via)
```

A useful agent policy is to use `recall()` during normal context assembly and
set `explain=True` only before a consequential action, when a surprising memory
wins, or when competing memories disagree. That keeps token cost low while
retaining a deterministic path from ranked memory back to stored provenance and
ranking evidence.
