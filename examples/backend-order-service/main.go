package main

import (
  "encoding/json"
  "log"
  "math/rand"
  "net/http"
  "os"
  "time"
)

type Order struct {
  ID     int     `json:"id"`
  Amount float64 `json:"amount"`
  UserID int     `json:"user_id"`
  Status string  `json:"status"`
}

func main() {
  mux := http.NewServeMux()

  mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(`{"status":"ok"}`))
  })

  mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
    orders := []Order{
      {1, 19.9, 1, "created"},
      {2, 199.0, 2, "shipped"},
    }
    jitter := time.Duration(rand.Intn(100)) * time.Millisecond
    time.Sleep(jitter)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(orders)
  })

  port := os.Getenv("PORT")
  if port == "" { port = "8082" }
  log.Printf("Order service listening on :%s", port)
  if err := http.ListenAndServe(":"+port, mux); err != nil { log.Fatal(err) }
}
