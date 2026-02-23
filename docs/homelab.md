# Homelab & Multi-Machine Setup

This guide covers running Sheldon across multiple machines with private networking and object storage.

## Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Your Private Network                     │
│                       (Tailscale)                           │
│                                                             │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │
│   │   VPS       │    │  GPU Server │    │  NAS/MinIO  │     │
│   │  Sheldon    │◄──►│   Ollama    │◄──►│   Storage   │     │
│   │  Headscale  │    │   Agent     │    │             │     │
│   └─────────────┘    └─────────────┘    └─────────────┘     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Use cases:**

- Run Ollama on a GPU machine at home, Sheldon on a cheap VPS
- Store backups and files on your NAS
- Add/remove machines without exposing ports to the internet

## Capabilities

| Capability               | Host (VPS)               | Remote (via Headscale)    |
| ------------------------ | ------------------------ | ------------------------- |
| **App Management**       |                          |                           |
| Deploy apps              | `deploy_app`             | -                         |
| Remove apps              | `remove_app`             | -                         |
| List apps                | `list_apps`              | -                         |
| App status/logs          | `app_status`, `app_logs` | -                         |
| Build images             | `build_image`            | -                         |
| **Container Management** |                          |                           |
| List all containers      | -                        | `list_containers`         |
| Restart/Stop/Start       | -                        | `restart_container`, etc. |
| View logs                | -                        | `container_logs`          |
| System stats             | -                        | `remote_status`           |
| **Ollama**               |                          |                           |
| Use for inference        | Direct                   | Via `ollama_host` config  |
| Pull models              | `pull_model`             | -                         |
| **Code**                 |                          |                           |
| Write code               | `write_code`             | -                         |
| **Storage**              |                          |                           |
| Files                    | MinIO tools              | Same (MinIO on VPS)       |
| Backup memory            | `backup_memory`          | Same                      |
| **Network**              |                          |                           |
| Create auth keys         | Human only               | N/A                       |

**Note:** On the host VPS, Sheldon manages apps he deploys (via `apps.yml`). On remote machines, Sheldon can manage _all_ containers including infrastructure.

---

## Headscale (Self-Hosted Tailscale)

Headscale is an open-source Tailscale control server. It creates a private WireGuard network between your machines.

### Why Headscale?

| Feature | Tailscale.com     | Headscale       |
| ------- | ----------------- | --------------- |
| Control | Tailscale Inc     | You             |
| Cost    | Free tier limits  | Free forever    |
| Privacy | They see metadata | Fully private   |
| Setup   | Easier            | Slightly harder |

### Setup

Headscale is included in the docker-compose. Enable it by setting:

```env
HEADSCALE_URL=https://hs.yourdomain.com
```

**DNS setup:**

```
A    hs.yourdomain.com    → your-vps-ip
```

### Creating Pre-Auth Keys

Pre-auth keys let machines join your network. Create them manually:

```bash
# SSH into your VPS
docker exec -it headscale headscale preauthkeys create --user default --expiration 1h
```

This outputs a key like `abc123...`. Use it with the invite script.

### Adding a Machine

**Step 1: Generate an auth key** (on your Sheldon VPS):

```bash
ssh root@your-vps-ip
docker exec headscale headscale preauthkeys create --user default --expiration 1h
# outputs: abc123...
```

**Step 2: Run the invite script** (on the new machine):

```bash
HEADSCALE_URL=https://hs.yourdomain.com AUTHKEY=abc123 \
  curl -fsSL https://raw.githubusercontent.com/{owner}/kora/main/core/scripts/invite.sh | sudo bash
```

This will:

1. Install Docker and Tailscale
2. Ask for a machine name
3. Join your Headscale network using the auth key
4. Start Ollama and homelab-agent

**Agent only** (no private networking, just container management):

```bash
curl -fsSL https://raw.githubusercontent.com/{owner}/kora/main/core/scripts/agent.sh | sudo bash
```

### Switching Ollama Host

Tell Sheldon:

```
"Switch ollama to gpu-server"
```

Or use the tool directly:

```
set_config ollama_host http://gpu-server:11434
```

### Managing Machines

Sheldon can manage containers on any connected machine:

| Command                         | What it does                  |
| ------------------------------- | ----------------------------- |
| "List containers on gpu-server" | Shows all Docker containers   |
| "Restart ollama on gpu-server"  | Restarts the ollama container |
| "Check if minio is running"     | Gets container status         |
| "Show ollama logs"              | Gets recent container logs    |

---

## MinIO (Object Storage)

MinIO provides S3-compatible storage for backups, files, and sharing.

### Setup

Add to your `.env`:

```env
STORAGE_ENABLED=true
STORAGE_ENDPOINT=minio:9000
STORAGE_ACCESS_KEY=minioadmin
STORAGE_SECRET_KEY=your-secret-key
STORAGE_USE_SSL=false
```

If running MinIO on a separate machine:

```env
STORAGE_ENDPOINT=nas.your-tailnet:9000
```

### Storage Buckets

Sheldon uses three buckets:

| Bucket            | Purpose                         |
| ----------------- | ------------------------------- |
| `sheldon-user`    | User files (documents, exports) |
| `sheldon-agent`   | Agent files (notes, artifacts)  |
| `sheldon-backups` | Memory database backups         |

### Tools

| Tool            | Description                                    |
| --------------- | ---------------------------------------------- |
| `upload_file`   | Store a file                                   |
| `download_file` | Retrieve a file                                |
| `list_files`    | List stored files                              |
| `delete_file`   | Remove a file                                  |
| `share_link`    | Generate temporary download URL (up to 7 days) |
| `fetch_url`     | Download from URL and store (up to 100MB)      |
| `backup_memory` | Backup Sheldon's memory database               |

### Examples

**Store a note:**

```
"Save this as notes/meeting.md: [content]"
```

**Share a file:**

```
"Give me a download link for exports/report.pdf"
```

**Backup memory:**

```
"Backup your memory"
```

**Archive a webpage:**

```
"Download https://example.com/doc.pdf and save it"
```

---

## Security Model

### Network Security

- All traffic between machines is encrypted (WireGuard)
- No ports exposed to the internet except VPS services
- Machines authenticate with pre-auth keys (single-use)

### Permission Boundaries

| Action               | Who can do it               |
| -------------------- | --------------------------- |
| Create pre-auth keys | Human only (via CLI)        |
| Join network         | Machine with valid key      |
| Manage containers    | Sheldon (via homelab-agent) |
| Access storage       | Sheldon (via MinIO)         |

### What Sheldon CAN'T Do

- Create network access tokens (human creates keys)
- Access machines not in the Tailnet
- Run arbitrary commands (only Docker container operations)
- Access the host filesystem (only Docker socket)

---

## Troubleshooting

### Machine can't join network

```bash
# Check Headscale is running
docker logs headscale

# Verify the pre-auth key is valid
docker exec headscale headscale preauthkeys list --user default
```

### Sheldon can't reach remote Ollama

```bash
# Check connectivity
tailscale ping gpu-server

# Check ollama is running
curl http://gpu-server:11434/api/tags
```

### Storage not working

```bash
# Check MinIO is running
docker logs minio

# Test connectivity
curl http://minio:9000/minio/health/live
```

---

## Architecture Notes

### Homelab Agent

The homelab-agent runs on each machine and exposes a simple HTTP API:

| Endpoint                     | Method | Description                      |
| ---------------------------- | ------ | -------------------------------- |
| `/health`                    | GET    | Health check                     |
| `/status`                    | GET    | System stats (CPU, memory, disk) |
| `/containers`                | GET    | List all containers              |
| `/containers/{name}`         | GET    | Container status                 |
| `/containers/{name}/restart` | POST   | Restart container                |
| `/containers/{name}/stop`    | POST   | Stop container                   |
| `/containers/{name}/start`   | POST   | Start container                  |
| `/containers/{name}/logs`    | GET    | Container logs                   |

### Port Conventions

| Port  | Service       |
| ----- | ------------- |
| 8080  | Homelab agent |
| 11434 | Ollama        |
| 9000  | MinIO API     |
| 9001  | MinIO Console |

### Docker Images

All images are published to GitHub Container Registry:

```
ghcr.io/{owner}/sheldon:latest
ghcr.io/{owner}/sheldon-coder-sandbox:latest
ghcr.io/{owner}/sheldon-browser-sandbox:latest
ghcr.io/{owner}/sheldon-homelab-agent:latest
```
