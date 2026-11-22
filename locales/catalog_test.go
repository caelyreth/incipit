package locales

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
)

// TestEveryLanguageSaysEveryMessage has each catalog hold the source's
// messages, no more, each one read whole: a message dropped at load reads
// as its key.
func TestEveryLanguageSaysEveryMessage(t *testing.T) {
	keys := func(lang string) []string {
		data, err := os.ReadFile(lang + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		var out []string
		for k := range m {
			if !strings.HasPrefix(k, "@") {
				out = append(out, k)
			}
		}
		slices.Sort(out)
		return out
	}
	source := keys("en")
	for _, lang := range Catalog.Languages() {
		if got := keys(lang); !slices.Equal(got, source) {
			t.Errorf("%s holds other messages than en", lang)
		}
		for _, key := range source {
			if _, found, ok := Catalog.Message(lang, key); !ok || found != lang {
				t.Errorf("%s: %s is not read", lang, key)
			}
		}
	}
}
