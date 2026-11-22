// Package phrases is the product's words for the editor's text keys: the
// label and description keys policies declare for themselves, their
// options and their commands. Settings, the keymap and the palette read
// them here, in the language the app shows; a key without words reads as
// itself.
//
// The words are the app's catalog's (package locales), each under its key
// in camel case, policies.core.description as policiesCoreDescription. A
// command a text names in braces, {core.break}, is a placeholder there,
// {core_break}, since a catalog's names cannot hold a dot; Text writes it
// back, so the text names the command as settings read it
// (app/settings.Phrase).
package phrases

import (
	"strings"

	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/i18n"
)

// Text is a key's words, or the key itself when it has none.
func Text(key string) string {
	name := catalogKey(key)
	m, _, ok := locales.Catalog.Message(i18n.Locale(), name)
	if !ok {
		return key
	}
	var args map[string]any
	if names := m.Args(); len(names) > 0 {
		args = make(map[string]any, len(names))
		for _, arg := range names {
			args[arg.Name] = "{" + strings.ReplaceAll(arg.Name, "_", ".") + "}"
		}
	}
	return locales.Catalog.Format(i18n.Locale(), name, args)
}

// Has reports whether a key has words.
func Has(key string) bool {
	_, _, ok := locales.Catalog.Message(i18n.Locale(), catalogKey(key))
	return ok
}

// catalogKey is the catalog's name for key: its parts after the first
// capitalized and joined.
func catalogKey(key string) string {
	parts := strings.Split(key, ".")
	for i, part := range parts[1:] {
		if part != "" {
			parts[i+1] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}
