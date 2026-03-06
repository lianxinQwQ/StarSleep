package main

import (
	"fmt"
	"os"
	"strings"
)

var currentLang = "zh"

// initI18n detects the system locale and sets the language
func initI18n() {
	lang := os.Getenv("LANG")
	if l := os.Getenv("LC_ALL"); l != "" {
		lang = l
	}
	if l := os.Getenv("LANGUAGE"); l != "" {
		lang = l
	}
	setLang(lang)
}

func setLang(lang string) {
	lang = strings.ToLower(lang)
	switch {
	case strings.HasPrefix(lang, "en"):
		currentLang = "en"
	default:
		currentLang = "zh"
	}
}

// T returns the translated string for the given key.
// If args are provided, fmt.Sprintf is used with the translated template.
// For fmt.Errorf with %w, call T without args and pass the result to fmt.Errorf.
func T(key string, args ...any) string {
	var m map[string]string
	switch currentLang {
	case "en":
		m = enMessages
	default:
		m = zhMessages
	}
	msg, ok := m[key]
	if !ok {
		msg = key
	}
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

// extractGlobalFlags extracts --lang from args and returns the remaining args
func extractGlobalFlags(args []string) []string {
	var remaining []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--lang" && i+1 < len(args) {
			setLang(args[i+1])
			i++
			continue
		}
		remaining = append(remaining, args[i])
	}
	return remaining
}
