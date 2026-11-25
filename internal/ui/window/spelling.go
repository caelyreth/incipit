package window

import (
	"github.com/caelyreth/neverland/internal/app/keymap"
	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/caelyreth/neverland/internal/editor/policies/spelling"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo"
)

// systemSpelling is the system's spelling checker, as the spelling policy
// asks it.
type systemSpelling struct{}

func (systemSpelling) Check(text string, grammar bool) []spelling.Issue {
	found := mygo.Spelling.Check(text, grammar)
	out := make([]spelling.Issue, len(found))
	for i, is := range found {
		out[i] = spelling.Issue{From: is.From, To: is.To, Grammar: is.Grammar}
	}
	return out
}

func (systemSpelling) Correction(text string, from, to int) string {
	return mygo.Spelling.Correction(text, from, to)
}

// spellingGuesses are the system's corrections for word, where it checks
// spelling.
func spellingGuesses(word string) []string { return mygo.Spelling.Guesses(word, 0, len(word)) }

func spellingPredict(text string, at int, cb func(string)) {
	mygo.Spelling.Predict(text, at, cb)
}

// useSystemSpelling has the spelling policy ask the system's checker,
// where there is one: before any program is compiled.
func useSystemSpelling() {
	if mygo.Spelling.Available() {
		spelling.Use(systemSpelling{})
	}
}

// The spelling options Edit › Spelling and Grammar turns on and off, as
// the system's apps keep them there; Settings shows them too.
var (
	checkSpellingKey   = policy.EnabledKey(spelling.ID)
	checkGrammarKey    = spelling.ID + ".grammar"
	correctSpellingKey = spelling.ID + ".correct"
	predictSpellingKey = spelling.ID + ".predict"
)

// spellingMenu is Edit's Spelling and Grammar, where the system has a
// checker.
func (a *AppState) spellingMenu(command func(keymap.CommandID) *mygo.MenuItem) []*mygo.MenuItem {
	if !mygo.Spelling.Available() {
		return nil
	}
	return []*mygo.MenuItem{mygo.Separator(), {
		Label: locales.MenuSpelling(),
		Submenu: []*mygo.MenuItem{
			command(keymap.CheckSpelling),
			command(keymap.CheckGrammar),
			command(keymap.CorrectSpelling),
			command(keymap.PredictSpelling),
		},
	}}
}

// optionOn is whether a policy's bool option is on, its default where
// the person has not set it.
func (a *AppState) optionOn(key string) bool {
	if v, ok := a.Product.Policy[key].(bool); ok {
		return v
	}
	for _, p := range a.Product.Program.Catalog().Policies {
		for _, o := range p.Options {
			if o.Key == key {
				on, _ := o.Default.(bool)
				return on
			}
		}
	}
	return false
}

// toggleOption turns a policy's bool option over, as Settings does.
func (a *AppState) toggleOption(key string) {
	a.Settings.Apply(appsettings.SetOption(key, !a.optionOn(key)))
}
