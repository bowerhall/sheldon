# Homelab & Multi-Machine Setup

This guide covers running Sheldon across multiple machines with private networking and object storage.

## Overview

```
┌───────────────────────────────────────────────────┐
│              Private Network (Headscale)          │
│                                                   │
│   VPS                      GPU Server             │
│   ┌─────────────┐         ┌─────────────┐         │
│   │   Sheldon   │◄───────►│   Ollama    │         │
│   │  Headscale  │         │    Agent    │         │
│   │   Ollama    │         └─────────────┘         │
│   │   MinIO     │                                 │
│   └─────────────┘                                 │
└───────────────────────────────────────────────────┘
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
docker exec headscale headscale preauthkeys create --user 1 --expiration 1h
```

This outputs a key like `hskey-auth-...`. Use it with the invite script.

> **Note:** Headscale uses numeric user IDs. User 1 is created automatically on first run. List users with `docker exec headscale headscale users list`.

### Adding a Machine

**Step 1: Generate an auth key** (on your Sheldon VPS):

```bash
ssh root@your-vps-ip
docker exec headscale headscale preauthkeys create --user 1 --expiration 1h
# outputs: hskey-auth-...
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

Set `OLLAMA_HOST` in Doppler to your GPU server's Headscale IP:

```
OLLAMA_HOST=http://100.64.0.5:11434
```

Then redeploy. The `ollama_host` config is locked from runtime changes for security (prevents rogue server attacks).

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

## Using Headscale for Personal Devices

Sheldon's Headscale isn't just for homelab-agents - it's a full Tailscale replacement for all your devices.

### Join Your Devices

```bash
# On any device with Tailscale installed
tailscale up --login-server=https://hs.yourdomain.com
```

Approve the device:
```bash
# On your VPS
docker exec headscale headscale nodes list
docker exec headscale headscale nodes register --user default --key nodekey:xxx
```

### What You Can Do

| Use Case | Example |
|----------|---------|
| SSH between devices | `ssh laptop.sheldon.local` |
| Access home NAS | `smb://nas.sheldon.local` |
| Remote desktop | Connect to `desktop.sheldon.local:3389` |
| Share files | Access any device's services |
| Exit node | Route traffic through home |

### Magic DNS

All devices get a `.sheldon.local` hostname:
- `laptop.sheldon.local`
- `phone.sheldon.local`
- `gpu-beast.sheldon.local`

---

## Access Control Lists (ACLs)

ACLs restrict which devices can access which services. By default, Sheldon deploys with an ACL that protects the homelab-agent API.

### Default ACL

```json
{
  "acls": [
    {"action": "accept", "src": ["tag:sheldon"], "dst": ["*:8080"]},
    {"action": "accept", "src": ["*"], "dst": ["*:*"]}
  ]
}
```

**Rule 1:** Only devices tagged `tag:sheldon` can reach port 8080 (homelab-agent)
**Rule 2:** All other traffic flows normally (full Tailscale functionality)

### Why This Matters

Without the ACL, any device on your Headscale network could manage containers on any machine. With it:
- Your laptop can access everything **except** homelab-agent APIs
- Only Sheldon VPS can manage containers remotely
- Normal networking (SSH, file sharing, etc.) is unaffected

### Tagging Nodes

The CI automatically tags the VPS on deploy. To manually tag:

```bash
# List nodes
docker exec headscale headscale nodes list

# Tag a node
docker exec headscale headscale nodes tag -i <node-id> -t tag:sheldon
```

### Custom ACLs

Edit `deploy/headscale/acl.json` to customize:

```json
{
  "acls": [
    // Only VPS can access homelab-agent
    {"action": "accept", "src": ["tag:sheldon"], "dst": ["*:8080"]},

    // Block phone from SSH
    {"action": "deny", "src": ["phone"], "dst": ["*:22"]},

    // Allow everything else
    {"action": "accept", "src": ["*"], "dst": ["*:*"]}
  ]
}
```

Push to main and the CI will deploy the new ACL.

---

## Security Model

### Network Security

- All traffic between machines is encrypted (WireGuard)
- No ports exposed to the internet except VPS services
- Machines authenticate with pre-auth keys (single-use)
- ACLs restrict homelab-agent access to Sheldon VPS only

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
docker exec headscale headscale preauthkeys list --user 1
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

---

## Raspberry Pi Hosting

Sheldon can run on a Raspberry Pi with Ollama offloaded to a more powerful homelab machine.

```
┌─────────────────────┐      ┌─────────────────────┐
│   Raspberry Pi 4    │      │   Homelab / NUC     │
│   (4GB, $55)        │◄────►│   (GPU or bigger)   │
│                     │      │                     │
│   - Sheldon         │  HS  │   - Ollama          │
│   - Traefik         │      │   - nomic-embed     │
│   - MinIO           │      │   - qwen2.5:3b      │
│   - Headscale       │      │                     │
└─────────────────────┘      └─────────────────────┘
        HS = Headscale (WireGuard mesh)
```

### Hardware Requirements

| Device | RAM | Storage | Notes |
|--------|-----|---------|-------|
| Pi 4 (2GB) | Minimum | USB SSD required | Tight, may swap |
| Pi 4 (4GB) | Recommended | USB SSD required | Comfortable headroom |
| Pi 4 (8GB) | Ideal | USB SSD required | Room for future features |
| Pi 5 | Any | USB SSD required | Faster builds, better I/O |

**Important:** SD cards are too slow and wear out quickly. Use a USB 3.0 SSD.

### Building for ARM64

The standard Docker images are multi-arch. If you need to build locally:

```bash
cd core
GOOS=linux GOARCH=arm64 go build -o bin/sheldon-arm64 ./cmd/sheldon
```

Or build the Docker image on the Pi:

```bash
docker build -t sheldon:local .
```

### Setup

1. **Flash Raspberry Pi OS (64-bit)** to your SSD
2. **Install Docker:**
   ```bash
   curl -fsSL https://get.docker.com | sh
   sudo usermod -aG docker $USER
   ```

3. **Set `OLLAMA_HOST`** to point to your homelab machine:
   ```env
   OLLAMA_HOST=http://100.64.0.5:11434
   ```

4. **Deploy** using the standard docker-compose with the ARM64 images

### Limitations

| Feature | Impact on Pi |
|---------|--------------|
| Browser sandbox | Heavy - Chromium uses 500MB+ RAM |
| Coder sandbox | Slow - compilation takes longer |
| Memory (SQLite) | Fine - low overhead |
| Telegram/Discord | Fine - minimal resources |
| MinIO | Fine - but use external disk |

### Tips

- **Disable browser tools** if not needed (saves RAM)
- **Use swap** (2-4GB) as safety net
- **Monitor temperature** - add a heatsink or fan
- **External MinIO** - consider running MinIO on NAS instead of Pi
