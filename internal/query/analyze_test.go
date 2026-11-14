package query

import (
	"fmt"
	"strings"
	"testing"
)

func terms(ts []Token) string {
	var b []string
	for _, t := range ts {
		b = append(b, fmt.Sprintf("%s@%d", t.Term, t.Pos))
	}
	return strings.Join(b, " ")
}

func TestWordsFoldCaseWidthAndMarks(t *testing.T) {
	got := terms(Tokens("Café ＦＵＬＬ naïve RÉSUMÉ", Indexed))
	if got != "cafe@1 full@2 naive@3 resume@4" {
		t.Fatalf("tokens %q", got)
	}
	if Fold("ガラス") != "がらす" || Fold("ｶﾞﾗｽ") != "がらす" {
		t.Fatalf("kana fold %q %q", Fold("ガラス"), Fold("ｶﾞﾗｽ"))
	}
	if Fold("が") == Fold("か") {
		t.Fatal("a voicing mark is a letter's own")
	}
}

func TestARunIndexesItsCharactersPairsAndWords(t *testing.T) {
	ts := Tokens("关系数据库 go", Indexed)
	has := map[string]int{}
	for _, tk := range ts {
		has[tk.Term] = tk.Pos
	}
	for term, pos := range map[string]int{"关": 1, "关系": 1, "系数": 2, "数据": 3, "据库": 4, "库": 5, "go": 6} {
		if has[term] != pos {
			t.Errorf("%s at %d, want %d (%s)", term, has[term], pos, terms(ts))
		}
	}
	for _, tk := range ts {
		if "关系数据库 go"[tk.Start:tk.End] == "" {
			t.Fatalf("token %v spans nothing", tk)
		}
	}
	// A dictionary word longer than a pair stands at its first character.
	word := Tokens("ラーメンを食べた", Indexed)
	if !strings.Contains(terms(word), "らーめん@1") {
		t.Fatalf("word %s", terms(word))
	}
}

func TestAQueriedRunIsItsPairsWithItsLastPlaceLeftOpen(t *testing.T) {
	if got := terms(Tokens("数据库 go", Queried)); got != "数据@1 据库@2 go@4" {
		t.Fatalf("queried %q", got)
	}
	if got := terms(Tokens("库", Queried)); got != "库@1" {
		t.Fatalf("one character %q", got)
	}
}
