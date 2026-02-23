package main

import (
	"log"
	"net/http"
	"os"

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
	log.Printf("datafog-api listening on %s", addr)
	if err := http.ListenAndServe(addr, h.Handler()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
