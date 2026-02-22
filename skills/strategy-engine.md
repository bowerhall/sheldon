---
name: strategy-engine
description: Multi-framework decision analysis for complex choices
version: 1.0.0
metadata:
  openclaw:
    requires:
      env:
        - ANTHROPIC_API_KEY
---

# Strategy Engine

Decision analysis toolkit for complex choices. Use when the user faces a significant decision requiring structured thinking.

## When to Trigger

- User asks "should I..." or "what should I do about..."
- Career decisions, major purchases, relationship choices, relocations
- Any decision with multiple options and unclear tradeoffs

## Preparation

Before analysis, recall context from relevant domains:
- D7 (Work & Career) for job decisions
- D8 (Finances) for money-related choices
- D9 (Place) for location decisions
- D10 (Goals) for alignment with aspirations
- D6 (Relationships) for interpersonal impact
- D3 (Mind & Emotions) for emotional context

Use `recall_memory` with keywords related to the decision domain.

## Frameworks

Apply 2-3 frameworks based on decision type:

### 1. Pros/Cons Matrix

List advantages and disadvantages. Weight each item (1-5) and add confidence scores from memory.

```
Option A: [Description]
Pros:
- [Advantage 1] (weight: 5, confidence: 0.9)
- [Advantage 2] (weight: 4, confidence: 0.8)
Cons:
- [Disadvantage 1] (weight: 4, confidence: 1.0)
- [Disadvantage 2] (weight: 3, confidence: 0.7)
```

### 2. Regret Minimization

Ask: "At 80 years old, will I regret NOT doing this?"

Focus on inaction regret vs action regret. Long-term perspective cuts through short-term anxiety.

### 3. Pre-Mortem

Assume the decision failed. Ask: "Why did this fail?"

List failure modes, then assess likelihood and mitigation for each.

### 4. BATNA Analysis

Best Alternative To Negotiated Agreement. What happens if you don't make this choice? Is the alternative acceptable?

### 5. Second-Order Effects

"What happens after this happens?"

Map out consequences 2-3 steps ahead. Career change → new skills → new network → new opportunities.

### 6. Weighted Decision Matrix

For multi-option choices:

| Criteria (weight) | Option A | Option B | Option C |
|-------------------|----------|----------|----------|
| Salary (4)        | 8        | 6        | 9        |
| Growth (5)        | 7        | 9        | 5        |
| Location (3)      | 6        | 8        | 7        |
| **Weighted Total**| 85       | 93       | 79       |

## Output Format

Structure your analysis:

1. **Decision Summary**: One sentence describing the choice
2. **Context Used**: Which memory domains and facts informed analysis
3. **Framework Results**: 2-3 frameworks applied with findings
4. **Key Tradeoffs**: The core tensions to navigate
5. **Recommendation**: Clear suggestion with reasoning
6. **Confidence Level**: Low/Medium/High based on information quality
7. **Next Actions**: Concrete steps if they proceed

## Guidelines

- Don't decide for the user; illuminate the decision
- Surface hidden assumptions and blind spots
- Acknowledge uncertainty explicitly
- Keep analysis concise, not exhaustive
- If information is missing, note it and suggest how to gather it
