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

// CommandOwner is the layer that must execute a command.
type CommandOwner uint8

const (
	// CommandOwnerEditor executes against the mounted document.
	CommandOwnerEditor CommandOwner = iota + 1
	// CommandOwnerHost executes in the window or application around it.
	CommandOwnerHost
)

// String returns a diagnostic owner name.
func (o CommandOwner) String() string {
	switch o {
	case CommandOwnerEditor:
		return "editor"
	case CommandOwnerHost:
		return "host"
	default:
		return "none"
	}
}

// CommandRequirements are context a command needs beyond policy enablement.
// Requirements gate availability; they do not decide dispatch or selection.
type CommandRequirements uint8

const (
	// RequiresDocument needs an active page and its session.
	RequiresDocument CommandRequirements = 1 << iota
	// RequiresTable needs the caret to stand in a table cell.
	RequiresTable
)

// Has reports whether requirements include want.
func (r CommandRequirements) Has(want CommandRequirements) bool {
	return r&want != 0
}

// Satisfied reports whether the current document and caret context meet r.
func (r CommandRequirements) Satisfied(document, table bool) bool {
	return (!r.Has(RequiresDocument) || document) &&
		(!r.Has(RequiresTable) || table)
}

// Command is a stable action declaration: its identity, its name, what it
// acts on, the policy that owns it, the layer that executes it, the
// context it needs, and for an editor command the intent it means. It says nothing about keys; the default keymap gives
// those, and a configuration may change them.
type Command struct {
	ID           kernel.CommandID
	Label        string
	Scope        Scope
	Policy       kernel.PolicyID
	Owner        CommandOwner
	Requirements CommandRequirements
	// Subject is the policy the command is about, when that is not its
	// owner: the CJK toggle belongs to the workspace, so it still works
	// while CJK refinement is off, but it is about CJK refinement.
	Subject kernel.PolicyID
	// Intent is what an editor command means; a host command has none.
	Intent Intent
}

// Edits reports whether the command is an editor intent.
func (c Command) Edits() bool { return c.Intent.Kind != 0 }

// About is the policy a person would look for this command under.
func (c Command) About() kernel.PolicyID {
	if c.Subject != "" {
		return c.Subject
	}
	return c.Policy
}

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
	ordered := make([]Command, 0, len(editorCommands)+len(surfaceCommands))
	ordered = append(ordered, editorCommands[:]...)
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
