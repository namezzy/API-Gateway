package main

import (
  "encoding/json"
  "log"
  "math/rand"
  "net/http"
  "os"
  "time"
)

type User struct {
  ID   int    `json:"id"`
  Name string `json:"name"`
  Role string `json:"role"`
}

func main() {
  mux := http.NewServeMux()

  mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(`{"status":"ok"}`))
  })

  mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
    users := []User{
      {1, "Alice", "admin"},
      {2, "Bob", "user"},
      {3, "Cathy", "user"},
    }
    jitter := time.Duration(rand.Intn(80)) * time.Millisecond
    time.Sleep(jitter)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(users)
  })

  port := os.Getenv("PORT")
  if port == "" { port = "8081" }
  log.Printf("User service listening on :%s", port)
  if err := http.ListenAndServe(":"+port, mux); err != nil { log.Fatal(err) }
}
