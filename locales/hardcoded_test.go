package locales

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The app says nothing in its code: every word it shows is a message of
// this catalog. TestNoWordsInTheCode reads the code that shows things and
// fails on a string that reads as words, outside the places a string is
// not shown: an identity, a key, a log line, an error's text for the log.
// A string that is shown and is the same in every language, a brand or a
// format's pattern, is listed in sameInEveryLanguage with why.

// shownFrom are the folders whose code shows things, from the repository's
// root.
var shownFrom = []string{"internal/ui", "internal/app/keymap", "internal/app/settings", "internal/app/workspace", "internal/app/fonts", "internal/app/phrases", "internal/text/query", "internal/app/export"}

// sameInEveryLanguage are the strings shown as they are in every language.
var sameInEveryLanguage = map[string]string{
	"Neverland":         "the app's name",
	"Noto Serif":        "a font family's name",
	"Noto Sans":         "a font family's name",
	"NewCMMath-Book":    "a font's name",
	"Ag":                "a sample of a typeface",
	"Starfield":         "a theme's key, its name a message",
	"Daybreak":          "a theme's key, its name a message",
	"CmdOrCtrl+Z":       "a menu accelerator",
	"CmdOrCtrl+Shift+Z": "a menu accelerator",
	"CmdOrCtrl":         "a menu accelerator's modifier",
	"Ctrl":              "a menu accelerator's modifier",
	"Super":             "a menu accelerator's modifier",
	"Alt":               "a menu accelerator's modifier",
	"Shift":             "a menu accelerator's modifier",
	"Plus":              "a menu accelerator's key",
	"Search Index":      "a folder's name on disk",
	"Hm":                "a date pattern's skeleton",
	"EEEE":              "a date pattern's skeleton",
	"MMMd":              "a date pattern's skeleton",
	"yMMMd":             "a date pattern's skeleton",
}

// notShown are the calls whose strings are not shown: logs, errors kept
// for the log, identities, environment and file names.
var notShown = map[string]bool{
	"Key": true, "ID": true, "Errorf": true, "New": true, "Printf": true, "Println": true, "Print": true,
	"Fatalf": true, "Fatal": true, "Panicf": true, "panic": true, "Getenv": true, "Setenv": true, "LookupEnv": true,
	"MustParseQuery": true, "mustKey": true, "KeyNamed": true, "Parse": true, "ParseChord": true, "NewChord": true,
	"WithFontID": true, "Font": true, "FontFeatures": true, "Join": true, "Command": true,
}

// notShownMethods are the methods whose strings are an identity or a
// text for the log: an error's, a key's, a value as code keeps it.
var notShownMethods = map[string]bool{"Error": true, "Key": true, "String": true}

// notShownFields are the fields whose strings are not shown.
var notShownFields = map[string]bool{
	"Key": true, "ID": true, "Icon": true, "Name": false, "Command": true, "Font": true, "Family": true,
	"Path": true, "Selector": true, "StateKey": true, "Group": false, "Policy": true, "Accelerator": true,
}

var wordy = regexp.MustCompile(`\p{L}{2,}`)

func TestNoWordsInTheCode(t *testing.T) {
	fset := token.NewFileSet()
	for _, dir := range shownFrom {
		err := filepath.WalkDir(filepath.Join("..", dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") ||
				strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_gen.go") || filepath.Base(path) == "catalog.go" {
				return err
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			for _, lit := range shownStrings(f) {
				s, _ := strconv.Unquote(lit.Value)
				if _, same := sameInEveryLanguage[s]; !same {
					t.Errorf("%s: %q reads as words: say it with a message of locales", fset.Position(lit.Pos()), s)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// shownStrings are f's strings that read as words and stand where they
// may be shown.
func shownStrings(f *ast.File) []*ast.BasicLit {
	var out []*ast.BasicLit
	var stack []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil || !readsAsWords(s) {
			return true
		}
		if shown(stack) {
			out = append(out, lit)
		}
		return true
	})
	return out
}

// readsAsWords is a string of words, not an identity: one with a space
// between letters, or one begun with a capital.
func readsAsWords(s string) bool {
	if !wordy.MatchString(s) {
		return false
	}
	if strings.ContainsAny(s, "/\\") && !strings.Contains(s, " ") {
		return false
	}
	first := []rune(strings.TrimSpace(s))
	return strings.Contains(strings.TrimSpace(s), " ") || len(first) > 0 && first[0] >= 'A' && first[0] <= 'Z'
}

// shown reports whether the string atop stack may be shown, by what holds
// it.
func shown(stack []ast.Node) bool {
	lit := stack[len(stack)-1]
	for _, n := range stack {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Recv != nil && notShownMethods[fn.Name.Name] {
			return false
		}
	}
	for i := len(stack) - 2; i >= 0; i-- {
		switch x := stack[i].(type) {
		case *ast.ImportSpec:
			return false
		case *ast.CallExpr:
			name := ""
			switch fn := x.Fun.(type) {
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			case *ast.Ident:
				name = fn.Name
			}
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
				if inner, ok := sel.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "Header" {
					return false
				}
			}
			return !notShown[name]
		case *ast.KeyValueExpr:
			if id, ok := x.Key.(*ast.Ident); ok && notShownFields[id.Name] {
				return false
			}
			if x.Key == lit {
				return false
			}
		case *ast.BinaryExpr:
			if x.Op == token.EQL || x.Op == token.NEQ {
				return false
			}
		case *ast.CaseClause:
			for _, e := range x.List {
				if e == stack[i+1] {
					return false
				}
			}
		case *ast.IndexExpr:
			if x.Index == stack[i+1] {
				return false
			}
		case *ast.FuncDecl:
			return true
		case *ast.GenDecl:
			return x.Tok != token.CONST
		}
	}
	return true
}
