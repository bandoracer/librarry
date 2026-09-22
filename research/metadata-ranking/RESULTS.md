# Frozen benchmark results

Captured 2026-09-21 UTC. Positive-target success, not general precision. Source IDs and acceptance criteria: [labels.json](labels.json).

| Query | Live first result | Candidate first result | Candidate target rank |
| --- | --- | --- | --- |
| percy jackson | Percy jackson (OL20658324W) | The Lightning Thief (OL492658W) | 1 |
| the lightning thief | The Lightning Thief / The Lost Hero / The Hidden Oracle (OL20140658W) | The Lightning Thief (OL492658W) | 1 |
| project hail mary | The Martian / Artemis / Project Hail Mary (OL36735881W) | Project Hail Mary (OL21745884W) | 1 |
| dune | Dune (OL19618275W) | Dune (OL893414W) | 1 |
| earthsea | Ursula K. Le Guin's A wizard of Earthsea (OL16835663W) | A Wizard of Earthsea (OL59798W) | 1 |
| rick riordan | Rick Riordan (OL30765A) | Rick Riordan (OL30765A) | 1 |
| percy jackson the ultimate guide | No result | Percy Jackson & the Olympians (OL17685821W) | 1 |
| the hobbit | The Hobbit (OL20683713W) | The Hobbit (OL27482W) | 1 |
| pride and prejudice | Pride and Prejudice (OL15165350W) | Pride and Prejudice (OL66554W) | 1 |
| murderbot | Murderbot Diaries (OL36737601W) | Network Effect (OL20735675W) | 1 |
| the left hand of darkness | Ursula K. Le Guin's the left hand of darkness (OL18955388W) | The Left Hand of Darkness (OL59800W) | 1 |
| 9781423103349 | No result | The Sea of Monsters (OL492646W) | 1 |
| the martian | The Martian (OL1940391W) | The Martian (OL17091839W) | 1 |
| watership down | Watership Down (OL15015120W) | Watership Down (OL2096536W) | 1 |
| dragonriders of pern | Dragonriders of Pern (OL8684296W) | Dragonflight (OL73387W) | 1 |
| a wizard of earthsea by ursula k le guin | No result | A Wizard of Earthsea (OL59798W) | 1 |

## Aggregate

| Method | Target first | Target in top 10 | MRR@10 |
| --- | --- | --- | --- |
| live | 2/16 | 3/16 | 0.146 |
| same_pool_rerank | 3/16 | 3/16 | 0.188 |
| title_language_fix | 11/16 | 12/16 | 0.708 |
| retrieval_only | 16/16 | 16/16 | 1.000 |
| candidate_v1 | 16/16 | 16/16 | 1.000 |
| without_cover | 16/16 | 16/16 | 1.000 |

Initial 7 development + 5 inspected validation queries; 4 further queries fetched after ranker weights were frozen. The four-query blind extension passes 4/4 for both retrieval-only and candidate v1.

Murderbot is relevant at #1, but All Systems Red is #5 in candidate v1 (#6 in provider order). Series-start navigation remains unsolved.

Exact ISBN case selects an edition containing 9781423103349. Work-level success does not generally certify edition or format correctness.
