package fonts

import "testing"

func TestCatalogLatinAndLanguages(t *testing.T) {
	langs := Languages()
	if len(langs) != 4 {
		t.Fatalf("Languages() = %d faces, want 4", len(langs))
	}
	for _, f := range langs {
		if f.IsLatin() {
			t.Errorf("Languages() included Latin face %q", f.ID)
		}
	}
	for _, style := range []Style{Serif, Sans} {
		lat, ok := Latin(style)
		if !ok || !lat.IsLatin() {
			t.Fatalf("Latin(%v) missing: %+v, %v", style, lat, ok)
		}
		if len(lat.Files) < 4 {
			t.Errorf("Latin(%v) expected upright+italic weights, got %d files", style, len(lat.Files))
		}
		if got, ok := LookupFamily(lat.Family); !ok || got.ID != lat.ID {
			t.Errorf("LookupFamily(%q) = %+v, %v", lat.Family, got, ok)
		}
	}
}

func TestResolveFamily(t *testing.T) {
	sc, _ := Find(Serif, "sc")
	cases := []struct {
		base, primary string
		fb            *Face
		want          string
	}{
		{"serif", "", nil, "serif"},
		{"sans-serif", "", &sc, "sans-serif, Noto Serif SC"},
		{"serif", "Georgia", nil, "Georgia, serif"},
		{"serif", "Noto Serif", &sc, "Noto Serif, Noto Serif SC, serif"},
		{"serif", "serif", nil, "serif"},
		{"serif", "Noto Serif SC", &sc, "Noto Serif SC, serif"},
	}
	for _, c := range cases {
		if got := ResolveFamily(c.base, c.primary, c.fb); got != c.want {
			t.Errorf("ResolveFamily(%q, %q, %v) = %q, want %q", c.base, c.primary, c.fb, got, c.want)
		}
	}
}
