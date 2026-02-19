# Phase 6: Self-Extend

**Timeline: 2-3 weeks**
**Depends on: Phase 3**
**Can run parallel with: Phase 4/5**
**Goal: Kora builds and deploys its own services to the cluster**

## Tasks

### 1. Container Builder (Week 1)
- Kora can write Dockerfiles, build images via kaniko
- Push to in-cluster registry
- Sandboxed: only kora-services namespace

### 2. Deployment Tool (Week 1-2)
- Tool in ToolRegistry: create Deployment + Service manifests
- Apply to kora-services namespace only (RBAC enforced)
- Resource limits enforced via ResourceQuota

### 3. Self-Built Services (Week 2-3)
- Kora builds and deploys 3+ services:
  - File sharing server
  - Personal dashboard
  - Webhook receiver
- Proves the self-extension loop works

## Success Criteria
- [ ] Kora has built and deployed 3+ services
- [ ] All services confined to kora-services namespace
- [ ] Resource limits prevent runaway containers
