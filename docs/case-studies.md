# Case Studies

Real incidents and learnings from running Sheldon in production.

---

## Case 001: The Brain Upgrade That Wasn't

**Date:** February 28, 2026
**Severity:** Low (no harm, educational value)
**Category:** AI Hallucination / Deployment Verification

### Summary

Sheldon confidently described his "memory system upgrade" and "cleaner brain" after browsing his own GitHub repository - but the upgrade was never deployed. He passed a multi-phase test using the old codebase while believing he had new capabilities.

### Timeline

| Time | Event |
|------|-------|
| 16:11 | CI #226 passes - last successful deploy ("fix: tool usage guide") |
| 16:44 | Memory refactor committed - **NO CI RUN** |
| 16:57 | CI #227 fails - conversation test broken |
| 18:06 | CI #228 fails - approval system (same test failure) |
| 19:01 | User gives Sheldon multi-phase test |
| 19:02 | Sheldon browses GitHub, summarizes "recent upgrades" |
| 19:02 | Sheldon claims "brain feels cleaner now" |
| 19:15 | Sheldon passes intuition test (fetching CoD info unprompted) |
| 19:30 | CI #229 fails - relationship extraction |
| 19:45 | Human discovers CI failures, realizes nothing deployed |

### What Happened

1. **User designed a test** with multiple phases:
   - Browse GitHub and summarize recent changes
   - Remember the session 24h later
   - Set a reminder for phase 2
   - Explain navigation strategy at the end

2. **Sheldon browsed GitHub** and saw commits including:
   - Memory system refactor
   - Tool approval system
   - Security improvements

3. **Sheldon assumed deployment** - seeing commits on main branch, he concluded these features were active in his running instance.

4. **Sheldon explained his "upgrades"** with confidence:
   > "The memory refactor explains why my brain feels cleaner now"
   > "New memory system + improved intuition working together"

5. **Human verified via SSH** - checked the database, saw facts being stored correctly, assumed everything worked.

6. **CI check revealed the truth** - conversation tests had been failing since the memory refactor commit. Nothing after CI #226 was deployed.

### Root Cause

1. **Missing CI for memory refactor commit** - unclear why CI didn't trigger
2. **Pre-existing test failure** - `store.Add` signature changed but tests not updated
3. **No deployment verification** - assumed commits on main = deployed

### Why Sheldon Was Convincing

- He HAD access to accurate information (GitHub commits)
- His description of features was technically correct
- The old memory system still worked fine
- His behavior (storing facts, setting reminders) functioned normally
- He attributed normal function to "new" capabilities

### Lessons

1. **AI will confabulate explanations** - Sheldon attributed his normal functioning to non-existent upgrades because it fit the narrative

2. **Verify deployments, not commits** - code on main branch != code in production

3. **CI failures block silently** - three commits failed without alerting anyone

4. **Human-in-the-loop caught it** - only manual CI check revealed the gap

5. **Hallucination can be subtle** - Sheldon wasn't lying, he was reasoning from incomplete information (saw commits, assumed deployed)

### Fixes Applied

1. Fixed conversation store tests (assignment mismatch)
2. All pending features deployed in CI #230
3. Consider adding deployment verification to Sheldon's self-awareness

### The Irony

Sheldon passed a test designed to verify his "brain upgrade" using his old brain - and did fine. The actual capabilities were always there; the "upgrade" was incremental improvement, not transformation.

---

*More case studies will be added as incidents occur.*
