package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		name := os.Getenv("APP_NAME")
		if name == "" {
			name = "sample-app"
		}
		fmt.Fprintf(w, "Hello from %s!\n", name)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	fmt.Println("Starting server on :8080")
	http.ListenAndServe(":8080", nil)
}
