# kora skills

coding patterns for claude code. kora injects relevant skills based on prompt keywords.

## usage

1. push this folder to your own repo
2. set `CODER_SKILLS_REPO` in kora config to your repo url
3. kora clones at startup and injects matching skills into prompts

## adding skills

create a new `.md` file with patterns. kora matches by filename keywords:

| file | triggers on |
|------|-------------|
| `go-api.md` | "go", "golang", "api" |
| `python-api.md` | "python", "fastapi", "flask" |
| `dockerfile.md` | "docker", "container", "image" |
| `k8s-manifest.md` | "kubernetes", "k8s", "deploy" |
| `general.md` | always included |

## updating

just push to your repo. restart kora pod to pull latest:

```bash
kubectl rollout restart deployment/kora -n kora
```

or wait for next pod restart.
