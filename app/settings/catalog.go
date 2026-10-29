package settings

import (
	"sort"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

const (
	policyEditorText       kernel.PolicyID = "editor.text"
	policyEditorMarkup     kernel.PolicyID = "editor.markup"
	policyEditorPairs      kernel.PolicyID = "editor.pairs"
	policyEditorCJK        kernel.PolicyID = "editor.cjk"
	policyEditorTypography kernel.PolicyID = "editor.typography"
	policyEditorHistory    kernel.PolicyID = "editor.history"
	policyEditorNavigation kernel.PolicyID = "editor.navigation"
	policyEditorWords      kernel.PolicyID = "editor.words"
	policyEditorSurface    kernel.PolicyID = "editor.surface"
)

type policyInfo struct {
	label       string
	description string
	order       int
	toggleable  bool
}

func infoForPolicy(id kernel.PolicyID) policyInfo {
	switch id {
	case policyEditorText:
		return policyInfo{"Core editing", "Text input, selection and deletion.", 4, false}
	case policyEditorPairs:
		return policyInfo{
			"Automatic pairs",
			"Complete, wrap, overtype and edit matching punctuation.",
			0,
			true,
		}
	case policyEditorMarkup:
		return policyInfo{
			"Emphasis",
			"Inline emphasis with generated markers. Text stays semantic.",
			1,
			true,
		}
	case policyEditorCJK:
		return policyInfo{
			"CJK refinement",
			"Refine punctuation and spacing in edited paragraphs.",
			2,
			true,
		}
	case policyEditorTypography:
		return policyInfo{
			"Typography",
			"Refine typed symbols and punctuation.",
			3,
			true,
		}
	case policyEditorHistory:
		return policyInfo{
			"Undo & redo",
			"Retain semantic edits and selection history.",
			4,
			true,
		}
	case policyEditorNavigation:
		return policyInfo{
			"Scope navigation",
			"Tab out of pairs or emphasis, or return to the last edit.",
			4,
			true,
		}
	case policyEditorWords:
		return policyInfo{
			"Word navigation",
			"Unicode word, sentence and paragraph selection.",
			4,
			true,
		}
	case policyEditorSurface:
		return policyInfo{
			"Workspace",
			"Visual navigation, clipboard, pages and application controls.",
			4,
			false,
		}
	default:
		return policyInfo{"Editor feature", "", 4, true}
	}
}

func labelForSetting(id kernel.SettingID) string {
	switch id {
	case "quotes":
		return "Pair quotation marks"
	case "underscores":
		return "Use underscore markers"
	case "allow-strong":
		return "Allow strong emphasis"
	case "overtype":
		return "Skip existing closers"
	case "edit-mates":
		return "Edit matching mates"
	case "contextual-quotes":
		return "Close quotes in context"
	case "punctuation":
		return "Refine punctuation"
	case "spacing":
		return "Space CJK and Latin text"
	case "symbols":
		return "Smart symbols"
	case "quote-placement":
		return "Punctuation around quotes"
	case "pairs":
		return "Tab out of pairs"
	case "emphasis":
		return "Tab out of emphasis"
	case "return-to-edit":
		return "Offer return after pointer moves"
	default:
		return "Option"
	}
}

func descriptionForSetting(id kernel.SettingID) string {
	switch id {
	case "quotes":
		return "Complete matching quotation marks while typing."
	case "underscores":
		return "Project emphasis with underscores instead of asterisks."
	case "allow-strong":
		return "Allow double-marker strong emphasis commands."
	case "overtype":
		return "Move past a matching closer instead of inserting it again."
	case "edit-mates":
		return "Update the matching boundary when editing a pair."
	case "contextual-quotes":
		return "Use surrounding text to decide whether a quote opens or closes."
	case "punctuation":
		return "Refine punctuation in the paragraph being edited."
	case "spacing":
		return "Insert spacing between CJK and Latin text where appropriate."
	case "symbols":
		return "Refine typed symbol sequences and punctuation."
	case "quote-placement":
		return "Refine punctuation around quotation marks."
	case "pairs":
		return "Offer the nearest enclosing pair as a Tab destination."
	case "emphasis":
		return "Offer the end of an emphasis scope as a Tab destination."
	case "return-to-edit":
		return "Offer a temporary return destination after moving away from an edit."
	default:
		return ""
	}
}

func commandLabel(command kernel.CommandID) string {
	for _, meta := range commandCatalog() {
		if meta.ID == command {
			return meta.Label
		}
	}
	return "Editor command"
}

func commandCatalog() []editor.Command {
	commands := make([]editor.Command, 0, 48)
	for _, command := range editor.AllTextCommands {
		commands = append(commands, command.Description())
	}
	for _, command := range editor.AllHistoryCommands {
		commands = append(commands, command.Description())
	}
	for _, command := range editor.AllWordCommands {
		commands = append(commands, command.Description())
	}
	commands = append(
		commands,
		editor.MarkupEmphasis.Description(),
		editor.MarkupStrong.Description(),
		editor.TabCommand(),
	)
	for _, command := range editor.SurfaceCommands() {
		commands = append(commands, command.Description())
	}
	return commands
}

func sortedPolicies(catalog kernel.PolicyCatalog) []kernel.PolicyDescriptor {
	policies := catalog.Policies()
	sort.SliceStable(policies, func(i, j int) bool {
		left, right := infoForPolicy(policies[i].ID), infoForPolicy(policies[j].ID)
		if left.order != right.order {
			return left.order < right.order
		}
		return policies[i].ID < policies[j].ID
	})
	return policies
}
