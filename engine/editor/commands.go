package editor

import "github.com/caelyreth/neverland/engine/kernel"

// Scope is what an action acts on, which says where pressing its key matters.
type Scope uint8

const (
	// ScopeDocument acts on the page being written, at its caret.
	ScopeDocument Scope = iota + 1
	// ScopePages opens, finds, switches and closes pages.
	ScopePages
	// ScopeWindow changes what the window shows around the page.
	ScopeWindow
	// ScopeApp opens the application's own tools.
	ScopeApp
)

// AllScopes is display order: closest to the text first.
var AllScopes = [...]Scope{ScopeDocument, ScopePages, ScopeWindow, ScopeApp}

// Command is a stable action declaration: its identity, its name, what it
// acts on and the policy that owns it. It says nothing about keys; the
// default keymap gives those, and a configuration may change them.
type Command struct {
	ID     kernel.CommandID
	Label  string
	Scope  Scope
	Policy kernel.PolicyID
	// Subject is the policy the command is about, when that is not its
	// owner: the CJK toggle belongs to the workspace, so it still works
	// while CJK refinement is off, but it is about CJK refinement.
	Subject kernel.PolicyID
}

// About is the policy a person would look for this command under.
func (c Command) About() kernel.PolicyID {
	if c.Subject != "" {
		return c.Subject
	}
	return c.Policy
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
	nestDeeperCommandID     kernel.CommandID = "writing.nest-deeper"
	nestShallowerCommandID  kernel.CommandID = "writing.nest-shallower"
)

const (
	// WordsPolicyID owns semantic word and selection-unit commands.
	WordsPolicyID kernel.PolicyID = "editor.words"
	// NavigationPolicyID owns scope and return-to-edit navigation.
	NavigationPolicyID kernel.PolicyID = "editor.navigation"
	// WritingPolicyID owns inline formatting commands.
	WritingPolicyID kernel.PolicyID = "editor.writing"
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
	meta := Command{Scope: ScopeDocument, Policy: WordsPolicyID}
	switch c {
	case WordPrevious:
		meta.ID, meta.Label = previousWordCommandID, "Previous word"
	case WordNext:
		meta.ID, meta.Label = nextWordCommandID, "Next word"
	case WordExtendPrevious:
		meta.ID, meta.Label = extendPreviousWordCommandID, "Extend to previous word"
	case WordExtendNext:
		meta.ID, meta.Label = extendNextWordCommandID, "Extend to next word"
	case WordDeletePrevious:
		meta.ID, meta.Label = deletePreviousWordCommandID, "Delete previous word"
	case WordDeleteNext:
		meta.ID, meta.Label = deleteNextWordCommandID, "Delete next word"
	case SelectWord:
		meta.ID, meta.Label = selectWordCommandID, "Select word"
	case SelectSentence:
		meta.ID, meta.Label = selectSentenceCommandID, "Select sentence"
	case SelectParagraph:
		meta.ID, meta.Label = selectParagraphCommandID, "Select paragraph"
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
		ID:     navigationTabCommandID,
		Label:  "Leave scope / return to edit",
		Scope:  ScopeDocument,
		Policy: NavigationPolicyID,
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
		return Command{ID: markupStrongCommandID, Label: "Toggle strong emphasis", Scope: ScopeDocument, Policy: WritingPolicyID}
	}
	return Command{ID: markupEmphasisCommandID, Label: "Toggle emphasis", Scope: ScopeDocument, Policy: WritingPolicyID}
}

// ID returns the stable command identity.
func (c MarkupCommand) ID() kernel.CommandID { return c.Description().ID }

// NestingCommand moves the blocks a selection covers a level deeper into
// the group they stand in, or a level out of it.
type NestingCommand uint8

const (
	NestDeeper NestingCommand = iota + 1
	NestShallower
)

// AllNestingCommands is the declaration order.
var AllNestingCommands = [...]NestingCommand{NestDeeper, NestShallower}

// Description returns stable command metadata.
func (c NestingCommand) Description() Command {
	if c == NestShallower {
		return Command{ID: nestShallowerCommandID, Label: "Nest shallower", Scope: ScopeDocument, Policy: WritingPolicyID}
	}
	return Command{ID: nestDeeperCommandID, Label: "Nest deeper", Scope: ScopeDocument, Policy: WritingPolicyID}
}

// ID returns the stable command identity.
func (c NestingCommand) ID() kernel.CommandID { return c.Description().ID }

// commandRegistry is every editor and host action, in catalog order, and an
// index by identity. It is built once; every catalog that lists actions (the
// settings pages, the palette, the menus) reads it instead of keeping a list
// of its own.
var commandRegistry = buildCommandRegistry()

type registry struct {
	ordered []Command
	byID    map[kernel.CommandID]int
}

func buildCommandRegistry() registry {
	ordered := make([]Command, 0, 64)
	for _, command := range AllTextCommands {
		ordered = append(ordered, command.Description())
	}
	ordered = append(ordered, MarkupEmphasis.Description(), MarkupStrong.Description())
	for _, command := range AllNestingCommands {
		ordered = append(ordered, command.Description())
	}
	for _, command := range AllTableCommands {
		ordered = append(ordered, command.Description())
	}
	for _, command := range AllHistoryCommands {
		ordered = append(ordered, command.Description())
	}
	ordered = append(ordered, TabCommand())
	for _, command := range AllWordCommands {
		ordered = append(ordered, command.Description())
	}
	for _, command := range surfaceCommands {
		ordered = append(ordered, command.Description())
	}
	byID := make(map[kernel.CommandID]int, len(ordered))
	for index, command := range ordered {
		byID[command.ID] = index
	}
	return registry{ordered: ordered, byID: byID}
}

// Commands returns every action declaration in catalog order.
func Commands() []Command {
	return append([]Command(nil), commandRegistry.ordered...)
}

// CommandFor resolves one action declaration by identity.
func CommandFor(id kernel.CommandID) (Command, bool) {
	index, ok := commandRegistry.byID[id]
	if !ok {
		return Command{}, false
	}
	return commandRegistry.ordered[index], true
}
