# sheldon skills

coding patterns for coder. sheldon injects relevant skills based on prompt keywords.

## usage

1. push this folder to your own repo
2. set `CODER_SKILLS_REPO` in sheldon config to your repo url
3. sheldon clones at startup and injects matching skills into prompts

## adding skills

create a new `.md` file with patterns. sheldon matches by filename keywords:

| file | triggers on |
|------|-------------|
| `go-api.md` | "go", "golang", "api" |
| `python-api.md` | "python", "fastapi", "flask" |
| `dockerfile.md` | "docker", "container", "image" |
| `compose.md` | "compose", "deploy", "traefik" |
| `general.md` | always included |

## updating

just push to your repo. restart sheldon to pull latest:

```bash
docker compose restart sheldon
```

or wait for next container restart.
