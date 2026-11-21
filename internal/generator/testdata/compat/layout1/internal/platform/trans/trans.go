// Package trans returns the texts the service shows to people, from
// lang/fa.json next to this file (embedded in the binary):
//
//	trans.Message("general:entity-not-found", "سفارش") // "سفارش مورد نظر یافت نشد."
//
// fa.json maps a key to its text, with %s where Message puts its arguments.
// Keys are grouped by a prefix, "<group>:<name>"; the test of this package
// fails when the code uses a key that is not in fa.json.
package trans

import (
	"embed"
	"encoding/json"
	"fmt"
)

// file is the language file Message reads.
const file = "lang/fa.json"

// langFiles are the language files.
//
//go:embed lang/*.json
var langFiles embed.FS

// phrases is read once, when the program starts, and never changed.
var phrases = load()

func load() map[string]string {
	data, err := langFiles.ReadFile(file)
	if err != nil {
		panic(fmt.Sprintf("trans: %s: %v", file, err))
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		panic(fmt.Sprintf("trans: %s: %v", file, err))
	}
	return m
}

// Message returns the text of key with args put in its %s, in order. A key that is
// not in the language file is returned as it is.
func Message(key string, args ...any) string {
	text, ok := phrases[key]
	if !ok {
		return key
	}
	if len(args) == 0 {
		return text
	}
	strs := make([]any, len(args))
	for i, arg := range args {
		strs[i] = fmt.Sprint(arg) // %s works for numbers too
	}
	return fmt.Sprintf(text, strs...)
}

// Has reports whether key is in the language file.
func Has(key string) bool {
	_, ok := phrases[key]
	return ok
}
