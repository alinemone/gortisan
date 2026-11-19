package generator

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// initialisms are written in capitals in Go names: user_id -> UserID.
var initialisms = map[string]string{
	"id": "ID", "ids": "IDs", "url": "URL", "uri": "URI", "api": "API",
	"http": "HTTP", "json": "JSON", "uuid": "UUID", "ip": "IP", "sql": "SQL",
	"sku": "SKU", "html": "HTML", "xml": "XML",
}

var identifierRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// snake turns "GetOrder", "get-order" or "get order" into "get_order".
func snake(s string) string {
	var b strings.Builder
	runes := []rune(strings.TrimSpace(s))
	for i, r := range runes {
		switch {
		case r == '-' || r == ' ' || r == '.':
			b.WriteRune('_')
		case unicode.IsUpper(r):
			prevLower := i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]))
			nextLower := i > 0 && i+1 < len(runes) && unicode.IsUpper(runes[i-1]) && unicode.IsLower(runes[i+1])
			if prevLower || nextLower {
				b.WriteRune('_')
			}
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(r)
		}
	}
	out := b.String()
	for strings.Contains(out, "__") {
		out = strings.ReplaceAll(out, "__", "_")
	}
	return strings.Trim(out, "_")
}

// pascal turns "user_id" into "UserID".
func pascal(s string) string {
	var b strings.Builder
	for _, word := range strings.Split(snake(s), "_") {
		if word == "" {
			continue
		}
		if upper, ok := initialisms[word]; ok {
			b.WriteString(upper)
			continue
		}
		b.WriteString(strings.ToUpper(word[:1]) + word[1:])
	}
	return b.String()
}

// camel turns "order_item" into "orderItem" and "id" into "id".
func camel(s string) string {
	p := pascal(s)
	if p == "" {
		return p
	}
	// Lower the leading run of capitals: "IDCard" -> "idCard", "URL" -> "url".
	runes := []rune(p)
	i := 0
	for i < len(runes) && unicode.IsUpper(runes[i]) {
		i++
	}
	if i > 1 && i < len(runes) {
		i-- // keep the capital that starts the next word
	}
	for j := 0; j < i; j++ {
		runes[j] = unicode.ToLower(runes[j])
	}
	return string(runes)
}

// plural is a small English pluralizer for table names; pass --table when it
// guesses wrong.
func plural(word string) string {
	switch {
	case strings.HasSuffix(word, "y") && len(word) > 1 && !strings.ContainsRune("aeiou", rune(word[len(word)-2])):
		return word[:len(word)-1] + "ies"
	case strings.HasSuffix(word, "s"), strings.HasSuffix(word, "x"), strings.HasSuffix(word, "z"),
		strings.HasSuffix(word, "ch"), strings.HasSuffix(word, "sh"):
		return word + "es"
	default:
		return word + "s"
	}
}

// identifier checks that name, after snake-casing, is a usable Go/SQL name.
func identifier(kind, name string) (string, error) {
	s := snake(name)
	if !identifierRE.MatchString(s) {
		return "", fmt.Errorf("invalid %s name %q: use letters, digits and underscores, starting with a letter", kind, name)
	}
	return s, nil
}
