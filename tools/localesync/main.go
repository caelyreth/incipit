// Command localesync keeps the app's catalogs in step with its source,
// locales/en.json: each other language holds every message the source
// does, in the source's order, and no other. A message a language has
// keeps its words; one it lacks is the source's, to be translated, with
// only the plural cases its language has (Chinese and Japanese have other
// alone, so the source's one case goes). Descriptions stay in the source.
// Run it from the repository's root after changing en.json, then
// `go tool mygo generate`:
//
//	go tool localesync
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/i18n"
)

func main() {
	dir := flag.String("dir", "locales", "the catalogs' folder")
	source := flag.String("source", "en", "the source language")
	flag.Parse()
	keys, messages, err := read(filepath.Join(*dir, *source+".json"))
	if err != nil {
		log.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(*dir, "*.json"))
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		lang := strings.TrimSuffix(filepath.Base(file), ".json")
		if lang == *source {
			continue
		}
		_, have, err := read(file)
		if err != nil {
			log.Fatal(err)
		}
		added, dropped := 0, 0
		var b bytes.Buffer
		b.WriteString("{\n  \"@@locale\": " + quote(lang))
		for _, k := range keys {
			v, ok := have[k]
			if !ok {
				v, added = cases(messages[k], lang), added+1
			}
			b.WriteString(",\n  " + quote(k) + ": " + quote(v))
		}
		for k := range have {
			if _, ok := messages[k]; !ok {
				dropped++
			}
		}
		b.WriteString("\n}\n")
		if err := os.WriteFile(file, b.Bytes(), 0o644); err != nil {
			log.Fatal(err)
		}
		log.Printf("%s: %d added from %s, %d no longer in it", lang, added, *source, dropped)
	}
}

// read is a catalog's message keys in order, and its messages.
func read(file string) ([]string, map[string]string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if _, err := dec.Token(); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", file, err)
	}
	var keys []string
	messages := map[string]string{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", file, err)
		}
		k := t.(string)
		var v any
		if err := dec.Decode(&v); err != nil {
			return nil, nil, fmt.Errorf("%s: %s: %w", file, k, err)
		}
		if s, ok := v.(string); ok && !strings.HasPrefix(k, "@") {
			keys = append(keys, k)
			messages[k] = s
		}
	}
	return keys, messages, nil
}

// quote is s as JSON writes it, its characters as they are.
func quote(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		log.Fatal(err)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// cases is message with the cases of its plurals lang has no category
// for left out; an exact case, as =0, stays.
func cases(message, lang string) string {
	has := map[string]bool{}
	for _, p := range i18n.PluralCategories(lang) {
		has[p.String()] = true
	}
	var out strings.Builder
	for i := 0; i < len(message); {
		k, end, ok := pluralCase(message, i)
		if ok && !has[k] {
			i = end
			continue
		}
		out.WriteByte(message[i])
		i++
	}
	return out.String()
}

// pluralCase reads a plural case at i, " one {…}", and is its keyword and
// where it ends.
func pluralCase(s string, i int) (string, int, bool) {
	if s[i] != ' ' {
		return "", 0, false
	}
	rest := s[i+1:]
	for _, k := range []string{"zero", "one", "two", "few", "many"} {
		if !strings.HasPrefix(rest, k+" {") {
			continue
		}
		j, depth := i+1+len(k)+2, 1
		for ; j < len(s) && depth > 0; j++ {
			switch s[j] {
			case '{':
				depth++
			case '}':
				depth--
			}
		}
		return k, j, depth == 0
	}
	return "", 0, false
}
