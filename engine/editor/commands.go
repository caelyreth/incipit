package editor

import "github.com/caelyreth/neverland/engine/kernel"

// Command is a stable editor command declaration.
type Command struct {
	ID         kernel.CommandID
	Label      string
	Key        string
	Modifiers  kernel.Modifiers
	Policy     kernel.PolicyID
	BindingCtx kernel.BindingContext
}

// Binding returns the normalized default binding for this command.
func (c Command) Binding() kernel.BindingTarget {
	return kernel.NewBindingTarget(c.BindingCtx, c.Key, c.Modifiers)
}

const (
	previousWordCommandID       kernel.CommandID = "word.previous"
	nextWordCommandID           kernel.CommandID = "word.next"
	extendPreviousWordCommandID kernel.CommandID = "word.extend-previous"
	extendNextWordCommandID     kernel.CommandID = "word.extend-next"
	deletePreviousWordCommandID kernel.CommandID = "word.delete-previous"
	deleteNextWordCommandID     kernel.CommandID = "word.delete-next"
	selectWordCommandID         kernel.CommandID = "word.select"
	selectSentenceCommandID     kernel.CommandID = "sentence.select"
	selectParagraphCommandID    kernel.CommandID = "paragraph.select"

	navigationTabCommandID kernel.CommandID = "navigation.tab"

	markupEmphasisCommandID kernel.CommandID = "markup.emphasis"
	markupStrongCommandID   kernel.CommandID = "markup.strong"
)

const (
	// WordsPolicyID owns semantic word and selection-unit commands.
	WordsPolicyID kernel.PolicyID = "editor.words"
	// NavigationPolicyID owns scope and return-to-edit navigation.
	NavigationPolicyID kernel.PolicyID = "editor.navigation"
	// MarkupPolicyID owns inline formatting commands.
	MarkupPolicyID kernel.PolicyID = "editor.markup"
	// PairsPolicyID owns pair insertion and mate editing.
	PairsPolicyID kernel.PolicyID = "editor.pairs"
	// CJKPolicyID owns line refinement after text insertion.
	CJKPolicyID kernel.PolicyID = "editor.cjk"
	// TypographyPolicyID owns typed punctuation convergence.
	TypographyPolicyID kernel.PolicyID = "editor.typography"
)

// WordCommand is a semantic word-navigation or selection command.
type WordCommand uint8

const (
	WordPrevious WordCommand = iota + 1
	WordNext
	WordExtendPrevious
	WordExtendNext
	WordDeletePrevious
	WordDeleteNext
	SelectWord
	SelectSentence
	SelectParagraph
)

// AllWordCommands is the declaration order used by the policy catalog.
var AllWordCommands = [...]WordCommand{
	WordPrevious,
	WordNext,
	WordExtendPrevious,
	WordExtendNext,
	WordDeletePrevious,
	WordDeleteNext,
	SelectWord,
	SelectSentence,
	SelectParagraph,
}

// Description returns stable command metadata.
func (c WordCommand) Description() Command {
	meta := Command{
		Policy:     WordsPolicyID,
		BindingCtx: kernel.EditorBindingContext,
	}
	switch c {
	case WordPrevious:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = previousWordCommandID, "Previous word", "left", kernel.ModifiersAlt
	case WordNext:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = nextWordCommandID, "Next word", "right", kernel.ModifiersAlt
	case WordExtendPrevious:
		meta.ID, meta.Label, meta.Key = extendPreviousWordCommandID, "Extend to previous word", "left"
		meta.Modifiers = kernel.ModifiersAlt | kernel.ModifiersShift
	case WordExtendNext:
		meta.ID, meta.Label, meta.Key = extendNextWordCommandID, "Extend to next word", "right"
		meta.Modifiers = kernel.ModifiersAlt | kernel.ModifiersShift
	case WordDeletePrevious:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = deletePreviousWordCommandID, "Delete previous word", "backspace", kernel.ModifiersAlt
	case WordDeleteNext:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = deleteNextWordCommandID, "Delete next word", "delete", kernel.ModifiersAlt
	case SelectWord:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = selectWordCommandID, "Select word", "d", kernel.ModifiersPrimary
	case SelectSentence:
		meta.ID, meta.Label, meta.Key = selectSentenceCommandID, "Select sentence", "s"
		meta.Modifiers = kernel.ModifiersAlt | kernel.ModifiersShift
	case SelectParagraph:
		meta.ID, meta.Label, meta.Key = selectParagraphCommandID, "Select paragraph", "p"
		meta.Modifiers = kernel.ModifiersAlt | kernel.ModifiersShift
	}
	return meta
}

// ID returns the stable command identity.
func (c WordCommand) ID() kernel.CommandID { return c.Description().ID }

// Label returns the product-facing command label.
func (c WordCommand) Label() string { return c.Description().Label }

// WordCommands returns all declarations in catalog order.
func WordCommands() []Command {
	commands := make([]Command, 0, len(AllWordCommands))
	for _, command := range AllWordCommands {
		commands = append(commands, command.Description())
	}
	return commands
}

// TabCommand declares the scope/return navigation command.
func TabCommand() Command {
	return Command{
		ID:         navigationTabCommandID,
		Label:      "Leave scope / return to edit",
		Key:        "tab",
		Policy:     NavigationPolicyID,
		BindingCtx: kernel.EditorBindingContext,
	}
}

// MarkupCommand is one semantic emphasis toggle.
type MarkupCommand uint8

const (
	MarkupEmphasis MarkupCommand = 1
	MarkupStrong   MarkupCommand = 2
)

// Level returns the semantic emphasis strength.
func (c MarkupCommand) Level() uint8 { return uint8(c) }

// Description returns stable command metadata.
func (c MarkupCommand) Description() Command {
	if c == MarkupStrong {
		return Command{
			ID:         markupStrongCommandID,
			Label:      "Toggle strong emphasis",
			Key:        "b",
			Modifiers:  kernel.ModifiersPrimary,
			Policy:     MarkupPolicyID,
			BindingCtx: kernel.EditorBindingContext,
		}
	}
	return Command{
		ID:         markupEmphasisCommandID,
		Label:      "Toggle emphasis",
		Key:        "i",
		Modifiers:  kernel.ModifiersPrimary,
		Policy:     MarkupPolicyID,
		BindingCtx: kernel.EditorBindingContext,
	}
}

// ID returns the stable command identity.
func (c MarkupCommand) ID() kernel.CommandID { return c.Description().ID }
