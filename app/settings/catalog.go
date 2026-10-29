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
			"Emphasize with {markup.emphasis}, or strongly with {markup.strong}. Markers are generated; the text stays semantic.",
			1,
			true,
		}
	case policyEditorCJK:
		return policyInfo{
			"CJK refinement",
			"Refine punctuation and spacing in edited paragraphs. Switch it from anywhere with {surface.cjk}.",
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
			"Step back with {history.undo} and forward with {history.redo} through edits and selections.",
			4,
			true,
		}
	case policyEditorNavigation:
		return policyInfo{
			"Scope navigation",
			"Press {navigation.tab} to leave a pair or emphasis, or to return to your last edit.",
			4,
			true,
		}
	case policyEditorWords:
		return policyInfo{
			"Word navigation",
			"Move and select by Unicode word, sentence and paragraph.",
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

// ScopeTitle names a group of the Keymap.
func ScopeTitle(scope editor.Scope) string {
	switch scope {
	case editor.ScopePages:
		return "Pages"
	case editor.ScopeWindow:
		return "Window"
	case editor.ScopeApp:
		return "App"
	default:
		return "Writing"
	}
}

// ScopeDescription says what a group's keys act on.
func ScopeDescription(scope editor.Scope) string {
	switch scope {
	case editor.ScopePages:
		return "Open, find, switch and close pages."
	case editor.ScopeWindow:
		return "Change what the window shows around your page."
	case editor.ScopeApp:
		return "Open Neverland's own tools."
	default:
		return "Act on the page you are writing, at the caret."
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
		return "Leave pairs"
	case "emphasis":
		return "Leave emphasis"
	case "return-to-edit":
		return "Return to your last edit"
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
		return "{navigation.tab} steps past the closer of the nearest enclosing pair."
	case "emphasis":
		return "{navigation.tab} steps past the end of the emphasis you are in."
	case "return-to-edit":
		return "After you click away from an edit, {navigation.tab} takes you back to it."
	default:
		return ""
	}
}

func commandLabel(command kernel.CommandID) string {
	if meta, ok := editor.CommandFor(command); ok {
		return meta.Label
	}
	return "Editor command"
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
