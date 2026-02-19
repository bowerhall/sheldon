# Strategy Engine

## Purpose

Multi-framework decision analysis toolkit. Triggered when router classifies a message as decision-type. Uses Opus model for depth.

## Frameworks

1. **Pros/Cons Matrix** — weighted, with confidence scores from koramem
2. **BATNA Analysis** — best alternative to negotiated agreement
3. **Regret Minimization** — "At 80, will I regret not doing this?"
4. **Weighted Decision Matrix** — criteria × weight → score
5. **Pre-Mortem** — "Imagine this failed. Why?"
6. **Second-Order Effects** — "What happens after this happens?"

## Context

Pulls from all relevant domains via koramem.Recall. A career decision might pull D7 (Career), D10 (Goals), D8 (Finances), D9 (Place), D3 (Mind), D6 (Relationships). Graph traversal surfaces connected entities and constraints.

## Output

Structured analysis with: framework results, cross-domain context used, clear recommendation, key tradeoffs, confidence level, suggested next actions.
