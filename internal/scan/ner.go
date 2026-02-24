package scan

import (
	"strings"
	"unicode"

	"github.com/datafog/datafog-api/internal/models"
)

// NER provides a lightweight, dictionary+heuristic named entity recognizer.
// It detects PERSON, ORGANIZATION, and LOCATION entities using:
//   - Title-cased word sequences (2+ words starting with uppercase)
//   - Contextual triggers ("Mr.", "Dr.", "Inc.", "Corp.", "in", "at")
//   - Dictionaries of common names, organizations, and locations
//
// This is intentionally simple: no ML, no cgo, no external deps.
// Accuracy is traded for zero-dependency deployment.

var nerConfidences = map[string]float64{
	"person":       0.70,
	"organization": 0.65,
	"location":     0.65,
}

// personTriggers precede person names.
var personTriggers = map[string]bool{
	"mr":      true,
	"mr.":     true,
	"mrs":     true,
	"mrs.":    true,
	"ms":      true,
	"ms.":     true,
	"dr":      true,
	"dr.":     true,
	"prof":    true,
	"prof.":   true,
	"sir":     true,
	"madam":   true,
	"captain": true,
	"capt":    true,
	"capt.":   true,
}

// orgSuffixes identify organization names.
var orgSuffixes = map[string]bool{
	"inc":          true,
	"inc.":         true,
	"corp":         true,
	"corp.":        true,
	"corporation":  true,
	"llc":          true,
	"llp":          true,
	"ltd":          true,
	"ltd.":         true,
	"co":           true,
	"co.":          true,
	"company":      true,
	"group":        true,
	"holdings":     true,
	"foundation":   true,
	"institute":    true,
	"university":   true,
	"association":  true,
	"technologies": true,
	"systems":      true,
	"partners":     true,
	"labs":         true,
	"studios":      true,
}

// locationTriggers precede location names.
var locationTriggers = map[string]bool{
	"in":    true,
	"at":    true,
	"from":  true,
	"near":  true,
	"city":  true,
	"state": true,
	"town":  true,
}

// commonFirstNames is a small set of frequently occurring first names.
var commonFirstNames = map[string]bool{
	"james": true, "john": true, "robert": true, "michael": true,
	"william": true, "david": true, "richard": true, "joseph": true,
	"thomas": true, "charles": true, "christopher": true, "daniel": true,
	"matthew": true, "anthony": true, "mark": true, "donald": true,
	"steven": true, "paul": true, "andrew": true, "joshua": true,
	"mary": true, "patricia": true, "jennifer": true, "linda": true,
	"elizabeth": true, "barbara": true, "susan": true, "jessica": true,
	"sarah": true, "karen": true, "nancy": true, "lisa": true,
	"margaret": true, "betty": true, "sandra": true, "ashley": true,
	"emily": true, "donna": true, "michelle": true, "dorothy": true,
	"alice": true, "jane": true, "alex": true, "sam": true,
	"benjamin": true, "alexander": true, "peter": true, "george": true,
	"edward": true, "henry": true, "jack": true, "oliver": true,
	"emma": true, "sophia": true, "ava": true, "isabella": true,
}

// wellKnownLocations covers major cities and countries.
var wellKnownLocations = map[string]bool{
	"new york": true, "los angeles": true, "chicago": true,
	"houston": true, "phoenix": true, "philadelphia": true,
	"san antonio": true, "san diego": true, "dallas": true,
	"san jose": true, "san francisco": true, "seattle": true,
	"denver": true, "boston": true, "nashville": true,
	"washington": true, "atlanta": true, "miami": true,
	"london": true, "paris": true, "tokyo": true,
	"berlin": true, "sydney": true, "toronto": true,
	"mumbai": true, "beijing": true, "shanghai": true,
	"singapore": true, "dubai": true, "amsterdam": true,
	"california": true, "texas": true, "florida": true,
	"new jersey": true, "virginia": true, "massachusetts": true,
	"united states": true, "united kingdom": true, "canada": true,
	"australia": true, "germany": true, "france": true,
	"japan": true, "china": true, "india": true, "brazil": true,
}

// NEREnabled controls whether the NER engine runs. Can be toggled via env var.
var NEREnabled = true

// ScanNER runs the heuristic NER engine over text and returns findings
// for person, organization, and location entities.
func ScanNER(text string, entityFilter []string) []models.ScanFinding {
	if !NEREnabled {
		return nil
	}

	requested := map[string]struct{}{}
	if len(entityFilter) > 0 {
		for _, name := range entityFilter {
			requested[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
		}
	}

	wantPerson := len(requested) == 0 || hasKey(requested, "person")
	wantOrg := len(requested) == 0 || hasKey(requested, "organization")
	wantLoc := len(requested) == 0 || hasKey(requested, "location")

	findings := make([]models.ScanFinding, 0)

	tokens := tokenize(text)

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]

		// Skip non-title-cased words (heuristic: NER entities start with uppercase)
		if !isTitleCase(tok.text) {
			continue
		}

		// Gather consecutive title-cased words
		span := []tokenInfo{tok}
		for j := i + 1; j < len(tokens); j++ {
			next := tokens[j]
			// Allow small connecting words within multi-word names
			if isConnector(next.text) && j+1 < len(tokens) && isTitleCase(tokens[j+1].text) {
				span = append(span, next)
				continue
			}
			if isTitleCase(next.text) {
				span = append(span, next)
			} else {
				break
			}
		}

		fullText := buildSpanText(text, span)
		start := span[0].start
		end := span[len(span)-1].end

		// Check what the preceding word is for context
		prevWord := ""
		if i > 0 {
			prevWord = strings.ToLower(tokens[i-1].text)
		}

		// Detect if first word in span is likely sentence-initial (not a proper name)
		sentenceInitial := i == 0 || isSentenceEnd(tokens[i-1].text)

		// Check the last word in the span for org suffixes
		lastWord := strings.ToLower(span[len(span)-1].text)

		// Classification using heuristics
		if wantOrg && len(span) >= 1 && orgSuffixes[lastWord] {
			// For orgs, trim sentence-initial words that are unlikely part of the name.
			// Walk forward from start to find the actual org name beginning.
			orgSpan := span
			if sentenceInitial && len(span) > 1 {
				firstLower := strings.ToLower(span[0].text)
				if !commonFirstNames[firstLower] && !orgSuffixes[firstLower] {
					orgSpan = span[1:]
				}
			}
			orgText := buildSpanText(text, orgSpan)
			findings = append(findings, models.ScanFinding{
				EntityType: "organization",
				Value:      orgText,
				Start:      orgSpan[0].start,
				End:        orgSpan[len(orgSpan)-1].end,
				Confidence: nerConfidences["organization"],
			})
			i += len(span) - 1
			continue
		}

		if wantPerson && personTriggers[prevWord] && len(span) >= 1 {
			findings = append(findings, models.ScanFinding{
				EntityType: "person",
				Value:      fullText,
				Start:      start,
				End:        end,
				Confidence: nerConfidences["person"] + 0.1, // higher confidence with trigger
			})
			i += len(span) - 1
			continue
		}

		if wantPerson && len(span) >= 2 {
			firstLower := strings.ToLower(span[0].text)
			if commonFirstNames[firstLower] {
				findings = append(findings, models.ScanFinding{
					EntityType: "person",
					Value:      fullText,
					Start:      start,
					End:        end,
					Confidence: nerConfidences["person"],
				})
				i += len(span) - 1
				continue
			}
		}

		if wantLoc {
			lowerFull := strings.ToLower(fullText)
			if wellKnownLocations[lowerFull] {
				findings = append(findings, models.ScanFinding{
					EntityType: "location",
					Value:      fullText,
					Start:      start,
					End:        end,
					Confidence: nerConfidences["location"] + 0.15, // dictionary match
				})
				i += len(span) - 1
				continue
			}
		}

		if wantLoc && locationTriggers[prevWord] && len(span) >= 1 {
			findings = append(findings, models.ScanFinding{
				EntityType: "location",
				Value:      fullText,
				Start:      start,
				End:        end,
				Confidence: nerConfidences["location"],
			})
			i += len(span) - 1
			continue
		}
	}

	return findings
}

type tokenInfo struct {
	text  string
	start int
	end   int
}

func tokenize(text string) []tokenInfo {
	tokens := make([]tokenInfo, 0)
	i := 0
	runes := []rune(text)
	n := len(runes)

	for i < n {
		// Skip whitespace
		for i < n && unicode.IsSpace(runes[i]) {
			i++
		}
		if i >= n {
			break
		}

		start := i
		// Collect word characters (letters, digits, apostrophes, periods for abbreviations)
		for i < n && !unicode.IsSpace(runes[i]) {
			i++
		}

		word := string(runes[start:i])
		// Strip trailing punctuation except periods (for abbreviations like "Mr.")
		trimmed := strings.TrimRight(word, ",;:!?\"')")
		if trimmed == "" {
			continue
		}
		endPos := start + len([]rune(trimmed))
		tokens = append(tokens, tokenInfo{
			text:  trimmed,
			start: byteOffset(text, start),
			end:   byteOffset(text, endPos),
		})
	}

	return tokens
}

func byteOffset(text string, runeIdx int) int {
	runes := []rune(text)
	if runeIdx >= len(runes) {
		return len(text)
	}
	return len(string(runes[:runeIdx]))
}

func isTitleCase(s string) bool {
	runes := []rune(s)
	if len(runes) == 0 {
		return false
	}
	return unicode.IsUpper(runes[0]) && len(runes) > 1
}

func isConnector(s string) bool {
	lower := strings.ToLower(s)
	return lower == "of" || lower == "the" || lower == "and" || lower == "de" || lower == "van" || lower == "von"
}

func buildSpanText(text string, span []tokenInfo) string {
	if len(span) == 0 {
		return ""
	}
	start := span[0].start
	end := span[len(span)-1].end
	if start < 0 || end > len(text) || start >= end {
		return ""
	}
	return text[start:end]
}

func isSentenceEnd(s string) bool {
	if len(s) == 0 {
		return false
	}
	last := s[len(s)-1]
	return last == '.' || last == '!' || last == '?'
}

func hasKey(m map[string]struct{}, key string) bool {
	_, ok := m[key]
	return ok
}
