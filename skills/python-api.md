---
name: python-api
description: Python API development with FastAPI
version: 1.0.0
metadata:
  openclaw:
    requires:
      bins:
        - python3
        - pip
---

# Python API Patterns

## Structure

- use FastAPI for APIs (unless Flask requested)
- single app.py for simple APIs
- use uvicorn as server

## Example

```python
from fastapi import FastAPI
import uvicorn

app = FastAPI()

@app.get("/health")
def health():
    return {"status": "ok"}

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8080)
```

## requirements.txt

```
fastapi==0.109.0
uvicorn==0.27.0
```
