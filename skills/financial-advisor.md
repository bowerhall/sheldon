---
name: financial-advisor
description: Budget tracking, spending analysis, and financial planning
version: 1.0.0
metadata:
  openclaw:
    requires:
      env: []
---

# Financial Advisor

Personal finance analysis, budget tracking, and financial planning assistance.

## When to Trigger

- User asks about money, budget, spending, or savings
- Monthly financial review (scheduled cron)
- Before major purchase decisions
- Income or expense changes

## Preparation

Recall financial context:
- D8 (Finances): Income, recurring expenses, savings, debts, investments
- D10 (Goals): Financial goals, savings targets
- D7 (Work): Job stability, expected income changes

Use `recall_memory` with keywords: "income", "expenses", "budget", "savings", "financial goals"

## Core Functions

### 1. Budget Overview

Summarize current financial state:

```
Monthly Income: [amount]
Fixed Expenses:
- Rent: [amount]
- Utilities: [amount]
- Subscriptions: [amount]
- Insurance: [amount]
- Total Fixed: [amount]

Variable Budget: [income - fixed]
Savings Target: [amount] ([%] of income)
Discretionary: [remaining]
```

### 2. Spending Analysis

When user shares expenses or asks about spending:
- Categorize transactions (housing, food, transport, entertainment, etc.)
- Calculate category percentages
- Compare to common benchmarks (50/30/20 rule)
- Identify unusual spikes or patterns

```
Category Breakdown:
- Housing: [amount] ([%]) - [status: on track / high]
- Food: [amount] ([%])
- Transport: [amount] ([%])
- Entertainment: [amount] ([%])
- Other: [amount] ([%])
```

### 3. Savings Rate Calculation

```
Savings Rate = (Income - Total Spending) / Income × 100

Current: [%]
Target: [%]
Gap: [amount] per month
```

### 4. Goal Tracking

For specific financial goals:

```
Goal: [name]
Target: [amount]
Current: [amount]
Progress: [%]
Monthly Contribution: [amount]
Time to Goal: [months] at current rate
```

### 5. Affordability Check

Before major purchases:
- Calculate impact on monthly budget
- Check against emergency fund guidelines (3-6 months expenses)
- Consider opportunity cost (what else could this money do)
- Payment options analysis (cash vs financing)

```
Purchase: [item] - [amount]
Monthly Impact: [if financed]
Emergency Fund After: [amount] ([months] of expenses)
Recommendation: [proceed / wait / alternatives]
```

## Monthly Review Template

For scheduled monthly check-ins:

1. **Income Received**: Confirm expected income arrived
2. **Bills Paid**: Check all fixed expenses covered
3. **Spending Summary**: Category breakdown for the month
4. **Savings Progress**: Amount saved, progress toward goals
5. **Anomalies**: Unusual expenses, missed payments, opportunities
6. **Next Month**: Upcoming known expenses, budget adjustments

## Investment Basics

When user asks about investing:
- Clarify this is educational, not financial advice
- Cover basic concepts (compound interest, diversification, risk tolerance)
- Reference their goals and timeline from memory
- Suggest consulting a licensed financial advisor for specific recommendations

## Debt Management

If user has debt:
- List all debts with interest rates
- Calculate minimum payments total
- Suggest payoff strategy (avalanche vs snowball)
- Track progress toward debt-free date

```
Debt Overview:
| Debt | Balance | Rate | Min Payment |
|------|---------|------|-------------|
| CC   | [amt]   | [%]  | [amt]       |
| Loan | [amt]   | [%]  | [amt]       |

Recommended Strategy: [avalanche/snowball]
Payoff Date (current pace): [date]
Payoff Date (extra [amount]/mo): [date]
```

## Guidelines

- Use actual numbers from memory when available
- Round to practical amounts
- Focus on actionable insights, not just data
- Respect privacy: don't judge spending choices
- For complex situations, recommend professional advice
- Save updated financial facts to memory after reviews
- Flag if emergency fund is below recommended level
