---
name: go-api
description: Go API development patterns
version: 1.0.0
metadata:
  openclaw:
    requires:
      bins:
        - go
---

# Go API Patterns

## Structure

- single main.go for simple APIs
- use net/http, no frameworks unless requested
- graceful shutdown with signal handling

## Endpoints

- health check at GET /health
- use http.ServeMux for routing

## Example

```go
package main

import (
    "encoding/json"
    "net/http"
    "os"
    "os/signal"
    "syscall"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
    })

    server := &http.Server{Addr: ":8080", Handler: mux}

    go func() {
        sig := make(chan os.Signal, 1)
        signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
        <-sig
        server.Close()
    }()

    server.ListenAndServe()
}
```
