# python api patterns

## structure
- use fastapi for apis (unless flask requested)
- single app.py for simple apis
- use uvicorn as server

## example
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
