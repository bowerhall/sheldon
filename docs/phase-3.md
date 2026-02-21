# Phase 3: Infrastructure

**Timeline: 3-5 days**
**Depends on: Phase 0**
**Can run parallel with: Phase 1 and 2**
**Goal: Production-grade infrastructure**

## Tasks

### 1. MinIO Deployment (Day 1)
- Deploy MinIO container
- Buckets: sheldon-backups, sheldon-exports, sheldon-artifacts
- Server-side encryption, separate credentials

### 2. Automated Backups (Day 1-2)
- Cron: daily SQLite snapshot (sheldon.db) → MinIO
- Retention: 7 daily, 4 weekly
- Test full restore procedure
- SQLite backup is trivial: VACUUM INTO or file copy during WAL checkpoint

### 3. Docker Network Isolation (Day 2)
- Separate networks for storage vs apps
- Verify container isolation

### 4. Network Policies (Day 3)
- Storage: only accessible from sheldon container
- Apps: egress to internet only

### 5. Monitoring (Day 4)
- Prometheus + Grafana
- Dashboards: pod resources, API costs, sheldonmem fact counts, domain coverage
- Alerts: high memory, budget threshold

## Success Criteria
- [ ] MinIO running, SSE enabled
- [ ] Daily SQLite snapshots to MinIO, restore tested
- [ ] RBAC verified
- [ ] Monitoring dashboard live
