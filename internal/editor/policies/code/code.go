// Package code is the code policy: a code block's text is coloured by
// the language its info string names, the word after its opening fence.
// What a language writes is data (langs.go); one scanner reads every
// language, so its state is small and comparable and a keystroke rescans
// only about the edit (§11.5).
package code

import (
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/profile"
)

// ID is the policy's id.
const ID = "code"

// class is what a piece of code is, as highlighting tells it apart.
type class uint8

const (
	classKeyword class = iota + 1
	classType
	classConstant
	classNumber
	classString
	classComment
)

// token is a piece of code of one class; its fact kind tells which.
type token struct{}

// Policy is the code policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	sc := scanner{kinds: [...]*policy.FactKind[token]{
		classKeyword:  policy.Facts[token](p, "keyword", policy.Styles(profile.GroupCodeKeyword)),
		classType:     policy.Facts[token](p, "type", policy.Styles(profile.GroupCodeType)),
		classConstant: policy.Facts[token](p, "constant", policy.Styles(profile.GroupCodeConstant)),
		classNumber:   policy.Facts[token](p, "number", policy.Styles(profile.GroupCodeNumber)),
		classString:   policy.Facts[token](p, "string", policy.Styles(profile.GroupCodeString)),
		classComment:  policy.Facts[token](p, "comment", policy.Styles(profile.GroupCodeComment)),
	}}
	var kinds []policy.FactID
	for _, k := range sc.kinds[1:] {
		kinds = append(kinds, k.Kind())
	}
	policy.Scan[state](p, sc, kinds, policy.Where(func(u *policy.Unit) bool {
		m, ok := u.Context.(profile.Marker)
		return ok && u.Owner.Kind() == profile.Code && lexiconOf(profile.CodeLanguage(m.Info)) != nil
	}))
}, policy.Label("policies.code"), policy.Description("policies.code.description"))

// Language is the language a code block is coloured as, by its info
// string; false where none is known.
func Language(n *doc.Node) (string, bool) {
	lang := profile.CodeLanguage(profile.CodeInfo(n))
	return lang, lexiconOf(lang) != nil
}
