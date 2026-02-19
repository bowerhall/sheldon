# Phase 3: Infrastructure

**Timeline: 3-5 days**
**Depends on: Phase 0**
**Can run parallel with: Phase 1 and 2**
**Goal: Production-grade infrastructure**

## Tasks

### 1. MinIO Deployment (Day 1)
- Deploy MinIO in kora-storage namespace
- Buckets: kora-backups, kora-exports, kora-artifacts
- Server-side encryption, separate credentials

### 2. Automated Backups (Day 1-2)
- CronJob: daily SQLite snapshot (kora.db) → MinIO
- Retention: 7 daily, 4 weekly
- Test full restore procedure
- SQLite backup is trivial: VACUUM INTO or file copy during WAL checkpoint

### 3. Full RBAC (Day 2)
- ServiceAccounts: kora-core, kora-deployer, kora-observer
- Verify privilege escalation impossible

### 4. Network Policies (Day 3)
- kora-storage: ingress only from kora-system
- kora-services: egress to internet only

### 5. Monitoring (Day 4)
- Prometheus + Grafana
- Dashboards: pod resources, API costs, koramem fact counts, domain coverage
- Alerts: high memory, budget threshold

## Success Criteria
- [ ] MinIO running, SSE enabled
- [ ] Daily SQLite snapshots to MinIO, restore tested
- [ ] RBAC verified
- [ ] Monitoring dashboard live
