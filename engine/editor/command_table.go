package editor

import "github.com/caelyreth/neverland/engine/kernel"

// Editor command identities. A command is a name for an intent a key, a
// menu item or the palette can carry; the identities are stable, as
// settings keep remaps under them.
const (
	MoveLeftCommandID            kernel.CommandID = "text.left"
	MoveRightCommandID           kernel.CommandID = "text.right"
	ExtendLeftCommandID          kernel.CommandID = "text.select-left"
	ExtendRightCommandID         kernel.CommandID = "text.select-right"
	LineStartCommandID           kernel.CommandID = "text.line-start"
	LineEndCommandID             kernel.CommandID = "text.line-end"
	ExtendLineStartCommandID     kernel.CommandID = "text.select-line-start"
	ExtendLineEndCommandID       kernel.CommandID = "text.select-line-end"
	DocumentStartCommandID       kernel.CommandID = "text.document-start"
	DocumentEndCommandID         kernel.CommandID = "text.document-end"
	ExtendDocumentStartCommandID kernel.CommandID = "text.select-document-start"
	ExtendDocumentEndCommandID   kernel.CommandID = "text.select-document-end"
	SelectAllCommandID           kernel.CommandID = "text.select-all"
	DismissCommandID             kernel.CommandID = "text.dismiss-selection"
	DeleteBackwardCommandID      kernel.CommandID = "text.backspace"
	DeleteForwardCommandID       kernel.CommandID = "text.delete"
	BreakCommandID               kernel.CommandID = "text.newline"
	SoftBreakCommandID           kernel.CommandID = "table.line-break"
	MoveUpCommandID              kernel.CommandID = "surface.up"
	MoveDownCommandID            kernel.CommandID = "surface.down"
	ExtendUpCommandID            kernel.CommandID = "surface.select-up"
	ExtendDownCommandID          kernel.CommandID = "surface.select-down"
	PreviousWordCommandID        kernel.CommandID = "word.previous"
	NextWordCommandID            kernel.CommandID = "word.next"
	ExtendPreviousWordCommandID  kernel.CommandID = "word.extend-previous"
	ExtendNextWordCommandID      kernel.CommandID = "word.extend-next"
	DeletePreviousWordCommandID  kernel.CommandID = "word.delete-previous"
	DeleteNextWordCommandID      kernel.CommandID = "word.delete-next"
	SelectWordCommandID          kernel.CommandID = "word.select"
	SelectSentenceCommandID      kernel.CommandID = "sentence.select"
	SelectParagraphCommandID     kernel.CommandID = "paragraph.select"
	LeaveCommandID               kernel.CommandID = "navigation.tab"
	ReturnCommandID              kernel.CommandID = "navigation.return"
	UndoCommandID                kernel.CommandID = "history.undo"
	RedoCommandID                kernel.CommandID = "history.redo"
	EmphasisCommandID            kernel.CommandID = "markup.emphasis"
	StrongCommandID              kernel.CommandID = "markup.strong"
	NestDeeperCommandID          kernel.CommandID = "writing.nest-deeper"
	NestShallowerCommandID       kernel.CommandID = "writing.nest-shallower"
	InsertTableCommandID         kernel.CommandID = "table.insert"
	TableRowAboveCommandID       kernel.CommandID = "table.row-above"
	TableRowBelowCommandID       kernel.CommandID = "table.row-below"
	TableColumnLeftCommandID     kernel.CommandID = "table.column-left"
	TableColumnRightCommandID    kernel.CommandID = "table.column-right"
	TableDeleteRowCommandID      kernel.CommandID = "table.delete-row"
	TableDeleteColumnCommandID   kernel.CommandID = "table.delete-column"
	TableDeleteCommandID         kernel.CommandID = "table.delete"
	TableHeaderCommandID         kernel.CommandID = "table.header"
)

// editorCommands is every editor command, in catalog order: the intent each
// names, and the policy it belongs to.
var editorCommands = [...]Command{
	editorCommand(MoveLeftCommandID, "Move left", TextPolicyID, Move(ByGrapheme, Backward, false)),
	editorCommand(MoveRightCommandID, "Move right", TextPolicyID, Move(ByGrapheme, Forward, false)),
	editorCommand(ExtendLeftCommandID, "Extend left", TextPolicyID, Move(ByGrapheme, Backward, true)),
	editorCommand(ExtendRightCommandID, "Extend right", TextPolicyID, Move(ByGrapheme, Forward, true)),
	editorCommand(LineStartCommandID, "Start of line", TextPolicyID, Move(ByLine, Backward, false)),
	editorCommand(LineEndCommandID, "End of line", TextPolicyID, Move(ByLine, Forward, false)),
	editorCommand(ExtendLineStartCommandID, "Extend to line start", TextPolicyID, Move(ByLine, Backward, true)),
	editorCommand(ExtendLineEndCommandID, "Extend to line end", TextPolicyID, Move(ByLine, Forward, true)),
	editorCommand(DocumentStartCommandID, "Start of document", TextPolicyID, Move(ByPage, Backward, false)),
	editorCommand(DocumentEndCommandID, "End of document", TextPolicyID, Move(ByPage, Forward, false)),
	editorCommand(ExtendDocumentStartCommandID, "Extend to document start", TextPolicyID, Move(ByPage, Backward, true)),
	editorCommand(ExtendDocumentEndCommandID, "Extend to document end", TextPolicyID, Move(ByPage, Forward, true)),
	editorCommand(MoveUpCommandID, "Move up", TextPolicyID, Move(ByRow, Backward, false)),
	editorCommand(MoveDownCommandID, "Move down", TextPolicyID, Move(ByRow, Forward, false)),
	editorCommand(ExtendUpCommandID, "Extend up", TextPolicyID, Move(ByRow, Backward, true)),
	editorCommand(ExtendDownCommandID, "Extend down", TextPolicyID, Move(ByRow, Forward, true)),
	editorCommand(SelectAllCommandID, "Select all", TextPolicyID, SelectUnit(ByScope)),
	editorCommand(DismissCommandID, "Dismiss selection", TextPolicyID, Dismiss),
	editorCommand(DeleteBackwardCommandID, "Delete backward", TextPolicyID, Delete(ByGrapheme, Backward)),
	editorCommand(DeleteForwardCommandID, "Delete forward", TextPolicyID, Delete(ByGrapheme, Forward)),
	editorCommand(BreakCommandID, "Insert newline", TextPolicyID, Break),
	editorCommand(SoftBreakCommandID, "Line break in block", TextPolicyID, SoftBreak),

	editorCommand(EmphasisCommandID, "Toggle emphasis", WritingPolicyID, Format(1)),
	editorCommand(StrongCommandID, "Toggle strong emphasis", WritingPolicyID, Format(2)),
	editorCommand(NestDeeperCommandID, "Nest deeper", WritingPolicyID, Nest(Forward)),
	editorCommand(NestShallowerCommandID, "Nest shallower", WritingPolicyID, Nest(Backward)),
	editorCommand(InsertTableCommandID, "Insert table", WritingPolicyID, TableIntent(TableInsert)),
	tableCommand(TableRowAboveCommandID, "Insert row above", TableRowAbove),
	tableCommand(TableRowBelowCommandID, "Insert row below", TableRowBelow),
	tableCommand(TableColumnLeftCommandID, "Insert column left", TableColumnLeft),
	tableCommand(TableColumnRightCommandID, "Insert column right", TableColumnRight),
	tableCommand(TableDeleteRowCommandID, "Delete row", TableDeleteRow),
	tableCommand(TableDeleteColumnCommandID, "Delete column", TableDeleteColumn),
	tableCommand(TableDeleteCommandID, "Delete table", TableDelete),
	tableCommand(TableHeaderCommandID, "Toggle header row", TableHeader),

	editorCommand(UndoCommandID, "Undo", HistoryPolicyID, Undo),
	editorCommand(RedoCommandID, "Redo", HistoryPolicyID, Redo),

	editorCommand(LeaveCommandID, "Leave scope / return to edit", NavigationPolicyID, Leave),
	editorCommand(ReturnCommandID, "Go back into scope", NavigationPolicyID, Return),

	editorCommand(PreviousWordCommandID, "Previous word", WordsPolicyID, Move(ByWord, Backward, false)),
	editorCommand(NextWordCommandID, "Next word", WordsPolicyID, Move(ByWord, Forward, false)),
	editorCommand(ExtendPreviousWordCommandID, "Extend to previous word", WordsPolicyID, Move(ByWord, Backward, true)),
	editorCommand(ExtendNextWordCommandID, "Extend to next word", WordsPolicyID, Move(ByWord, Forward, true)),
	editorCommand(DeletePreviousWordCommandID, "Delete previous word", WordsPolicyID, Delete(ByWord, Backward)),
	editorCommand(DeleteNextWordCommandID, "Delete next word", WordsPolicyID, Delete(ByWord, Forward)),
	editorCommand(SelectWordCommandID, "Select word", WordsPolicyID, SelectUnit(ByWord)),
	editorCommand(SelectSentenceCommandID, "Select sentence", WordsPolicyID, SelectUnit(BySentence)),
	editorCommand(SelectParagraphCommandID, "Select paragraph", WordsPolicyID, SelectUnit(ByParagraph)),
}

func editorCommand(id kernel.CommandID, label string, policy kernel.PolicyID, intent Intent) Command {
	return Command{
		ID: id, Label: label, Scope: ScopeDocument, Policy: policy,
		Owner: CommandOwnerEditor, Requirements: RequiresDocument, Intent: intent,
	}
}

func tableCommand(id kernel.CommandID, label string, op TableOp) Command {
	command := editorCommand(id, label, WritingPolicyID, TableIntent(op))
	command.Requirements |= RequiresTable
	return command
}

// policyCommands are the commands a policy declares, in catalog order.
func policyCommands(policy kernel.PolicyID) []kernel.CommandID {
	ids := make([]kernel.CommandID, 0)
	for _, command := range editorCommands {
		if command.Policy == policy {
			ids = append(ids, command.ID)
		}
	}
	return ids
}

// CommandForIntent is the command naming an intent, for its label and keys.
func CommandForIntent(intent Intent) (Command, bool) {
	for _, command := range editorCommands {
		if command.Intent.Kind == intent.Kind && command.Intent.By == intent.By &&
			command.Intent.Dir == intent.Dir && command.Intent.Extend == intent.Extend &&
			command.Intent.Soft == intent.Soft && command.Intent.Level == intent.Level &&
			command.Intent.Table == intent.Table {
			return command, true
		}
	}
	return Command{}, false
}
