# go api patterns

## structure
- single main.go for simple apis
- use net/http, no frameworks unless requested
- graceful shutdown with signal handling

## endpoints
- health check at GET /health returning {"status": "ok"}
- use http.ServeMux for routing

## example
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
