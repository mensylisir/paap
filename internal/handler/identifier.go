package handler

import (
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

var identifierInvalidChars = regexp.MustCompile(`[^a-z0-9-]+`)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeIdentifier(input, fallbackPrefix string, maxLen int) string {
	candidate := strings.ToLower(strings.TrimSpace(input))
	candidate = strings.ReplaceAll(candidate, "_", "-")
	candidate = identifierInvalidChars.ReplaceAllString(candidate, "-")
	candidate = strings.Trim(candidate, "-")
	if candidate == "" || candidate[0] < 'a' || candidate[0] > 'z' {
		candidate = fallbackPrefix
	}
	if maxLen > 0 && len(candidate) > maxLen {
		candidate = strings.Trim(candidate[:maxLen], "-")
	}
	if candidate == "" {
		return fallbackPrefix
	}
	return candidate
}

func uniqueIdentifier(db *gorm.DB, base string, maxLen int, exists func(*gorm.DB, string) (bool, error)) (string, error) {
	return uniqueIdentifierWithFallback(db, base, "item", maxLen, exists)
}

func uniqueIdentifierWithFallback(db *gorm.DB, base, fallbackPrefix string, maxLen int, exists func(*gorm.DB, string) (bool, error)) (string, error) {
	base = normalizeIdentifier(base, fallbackPrefix, maxLen)
	candidate := base
	for i := 2; ; i++ {
		found, err := exists(db, candidate)
		if err != nil {
			return "", err
		}
		if !found {
			return candidate, nil
		}
		suffix := fmt.Sprintf("-%d", i)
		prefix := base
		if maxLen > 0 && len(prefix)+len(suffix) > maxLen {
			prefix = strings.Trim(prefix[:maxLen-len(suffix)], "-")
		}
		if prefix == "" {
			prefix = fallbackPrefix
		}
		candidate = prefix + suffix
	}
}
