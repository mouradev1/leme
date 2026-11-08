// Package i18n picks Portuguese or English for messages the backend shows
// to the user (errors, diagnoses). The UI sets the language at startup and
// whenever the user changes it.
package i18n

import "sync/atomic"

var english atomic.Bool

// Set selects the language: "en" for English, anything else Portuguese.
func Set(lang string) { english.Store(lang == "en") }

// English reports whether messages should be in English.
func English() bool { return english.Load() }

// T returns the Portuguese or the English text for the current language.
func T(pt, en string) string {
	if english.Load() {
		return en
	}
	return pt
}
