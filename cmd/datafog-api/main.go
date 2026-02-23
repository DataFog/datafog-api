package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/datafog/datafog-api/internal/policy"
	"github.com/datafog/datafog-api/internal/receipts"
	"github.com/datafog/datafog-api/internal/server"
)

func main() {
	policyPath := getenv("DATAFOG_POLICY_PATH", "config/policy.json")
	receiptPath := getenv("DATAFOG_RECEIPT_PATH", "datafog_receipts.jsonl")
	addr := getenv("DATAFOG_ADDR", ":8080")

	policyData, err := policy.LoadPolicyFromFile(policyPath)
	if err != nil {
		log.Fatalf("load policy: %v", err)
	}

	store, err := receipts.NewReceiptStore(receiptPath)
	if err != nil {
		log.Fatalf("init receipts: %v", err)
	}

	h := server.New(policyData, store, log.Default())
	srv := &http.Server{
		Addr:              addr,
		Handler:           h.Handler(),
		ReadTimeout:       getenvDuration("DATAFOG_READ_TIMEOUT", 5*time.Second),
		ReadHeaderTimeout: getenvDuration("DATAFOG_READ_HEADER_TIMEOUT", 2*time.Second),
		WriteTimeout:      getenvDuration("DATAFOG_WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:       getenvDuration("DATAFOG_IDLE_TIMEOUT", 30*time.Second),
		MaxHeaderBytes:    1 << 20, // 1 MiB
		ErrorLog:          log.Default(),
	}

	log.Printf("datafog-api listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server failed: %v", err)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		log.Printf("invalid duration for %s=%q, using fallback %s", key, value, fallback)
		return fallback
	}
	return parsed
}
