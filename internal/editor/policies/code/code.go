// Package code is the code policy: a code block's text is coloured by
// the language its info string names, the word after its opening fence.
// What a language writes is data (langs.go); one scanner reads every
// language, so its state is small and comparable and a keystroke rescans
// only about the edit (§11.5).
package code

import (
	"strings"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
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

	// Return in code starts the new line where the line it leaves starts,
	// a level in after an opening bracket.
	policy.On(p, "indent-new-line", policy.Intent[vocab.Break](), policy.Node(profile.Code).Own(), newLine)
}, policy.Label("policies.code"), policy.Description("policies.code.description"))

// closers are the brackets a line ending in an opener is a level in after,
// and the closer that ends that level.
var closers = map[byte]byte{'{': '}', '(': ')', '[': ']'}

// newLine breaks the line at the selection, the new line indented as the
// line it leaves: one level further after an opening bracket, and with
// the bracket's closer right after the caret, that closer put on a line of
// its own at the line's indentation. Without indentation to carry, the
// line break is core's, which leaves the block from its empty last lines.
func newLine(c *policy.Ctx, _ vocab.Break, _ policy.At[*doc.Node]) policy.Result {
	from, to := c.Range()
	in, ok := c.Inline(from.Ref())
	if !ok || to.Ref() != from.Ref() {
		return policy.Pass
	}
	text := in.Flat()
	start := strings.LastIndexByte(text[:from.Offset], '\n') + 1
	head := text[start:from.Offset]
	indent := head[:len(head)-len(strings.TrimLeft(head, " \t"))]
	before := strings.TrimRight(head, " \t")
	next := indent
	var after string
	if n := len(before); n > 0 {
		if closer, ok := closers[before[n-1]]; ok {
			next += profile.SourceIndent
			if to.Offset < len(text) && text[to.Offset] == closer {
				after = "\n" + indent
			}
		}
	}
	if next == "" {
		return policy.Pass
	}
	line := "\n" + next
	res := policy.Do(vocab.Ops(doc.EditText{At: from.Ref(), From: from.Offset, To: to.Offset,
		Items: []doc.Item{doc.TextItem(line+after, nil)}}))
	sel := doc.Collapsed(doc.CaretAt(doc.At(from.Ref(), from.Offset+len(line))))
	res.Selection = &sel
	return res
}

// Language is the language a code block is coloured as, by its info
// string; false where none is known.
func Language(n *doc.Node) (string, bool) {
	lang := profile.CodeLanguage(profile.CodeInfo(n))
	return lang, lexiconOf(lang) != nil
}
