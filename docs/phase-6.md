# Phase 6: Self-Extend

**Timeline: 2-3 weeks**
**Depends on: Phase 3**
**Can run parallel with: Phase 4/5**
**Goal: Sheldon builds and deploys its own services**

## Tasks

### 1. Container Builder (Week 1)
- Sheldon can write Dockerfiles, build images
- Push to GitHub Container Registry
- Isolated via Docker Compose networks

### 2. Deployment Tool (Week 1-2)
- Tool in ToolRegistry: generate docker-compose entries
- Auto-configure Traefik labels for routing
- Resource limits via Docker Compose

### 3. Self-Built Services (Week 2-3)
- Sheldon builds and deploys 3+ services:
  - File sharing server
  - Personal dashboard
  - Webhook receiver
- Proves the self-extension loop works

## Success Criteria
- [ ] Sheldon has built and deployed 3+ services
- [ ] All services on sheldon-net network
- [ ] Resource limits prevent runaway containers
