package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/datafog/datafog-api/internal/policy"
	"github.com/datafog/datafog-api/internal/receipts"
	"github.com/datafog/datafog-api/internal/server"
)

func main() {
	policyPath := getenv("DATAFOG_POLICY_PATH", "config/policy.json")
	receiptPath := getenv("DATAFOG_RECEIPT_PATH", "datafog_receipts.jsonl")
	apiToken := getenv("DATAFOG_API_TOKEN", "")
	addr := getenv("DATAFOG_ADDR", ":8080")
	rateLimitRPS := getenvInt("DATAFOG_RATE_LIMIT_RPS", 0)
	shutdownTimeout := getenvDuration("DATAFOG_SHUTDOWN_TIMEOUT", 10*time.Second)

	policyData, err := policy.LoadPolicyFromFile(policyPath)
	if err != nil {
		log.Fatalf("load policy: %v", err)
	}

	store, err := receipts.NewReceiptStore(receiptPath)
	if err != nil {
		log.Fatalf("init receipts: %v", err)
	}

	h := server.New(policyData, store, log.Default(), apiToken, rateLimitRPS)
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
	done := make(chan error, 1)
	go func() {
		done <- srv.ListenAndServe()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	case <-sig:
		log.Printf("shutdown signal received")
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
			if closeErr := srv.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
				log.Printf("forced close failed: %v", closeErr)
			}
		}
		if err := <-done; err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server stopped with error: %v", err)
		}
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

func getenvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		log.Printf("invalid integer for %s=%q, using fallback %d", key, value, fallback)
		return fallback
	}
	return parsed
}
