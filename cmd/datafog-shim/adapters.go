package main

import "github.com/datafog/datafog-api/internal/adapters"

type adapterSpec = adapters.Spec

var defaultAdapters = adapters.DefaultAdapters

func resolveAdapter(raw string, command string) string {
	return adapters.Resolve(raw, command)
}

func canonicalAdapter(value string) (string, bool) {
	return adapters.Canonical(value)
}

func normalizeAdapter(raw string) string {
	return adapters.Normalize(raw)
}

func knownAdapters() []adapterSpec {
	return adapters.KnownAdapters()
}
