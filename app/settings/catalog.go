package settings

import (
	"sort"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

const (
	policyEditorText       kernel.PolicyID = "editor.text"
	policyEditorWriting    kernel.PolicyID = "editor.writing"
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
	// order places the policy's section on the Editor page: Writing
	// Profile, Typography, Automatic pairs, Scope navigation, Word
	// navigation, CJK refinement, Undo & redo, then the rest.
	order      int
	toggleable bool
}

func infoForPolicy(id kernel.PolicyID) policyInfo {
	switch id {
	case policyEditorText:
		return policyInfo{"Core editing", "Text input, selection and deletion.", 7, false}
	case policyEditorPairs:
		return policyInfo{
			"Automatic pairs",
			"Complete, wrap, overtype and edit matching punctuation.",
			2,
			true,
		}
	case policyEditorWriting:
		return policyInfo{
			"Writing Profile",
			"Type Markdown and it becomes what it marks. Headings, lists, quotes, code, rules and emphasis are always part of a page; these choose what typing turns into them.",
			0,
			true,
		}
	case policyEditorCJK:
		return policyInfo{
			"CJK refinement",
			"Refine punctuation and spacing in edited paragraphs. Switch it from anywhere with {surface.cjk}.",
			5,
			true,
		}
	case policyEditorTypography:
		return policyInfo{
			"Typography",
			"Refine typed symbols and punctuation.",
			1,
			true,
		}
	case policyEditorHistory:
		return policyInfo{
			"Undo & redo",
			"Step back with {history.undo} and forward with {history.redo} through edits and selections.",
			6,
			true,
		}
	case policyEditorNavigation:
		return policyInfo{
			"Scope navigation",
			"Press {navigation.tab} to leave a pair or emphasis, or to return to your last edit.",
			3,
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
			7,
			false,
		}
	default:
		return policyInfo{"Editor feature", "", 7, true}
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
	case "asterisks":
		return "Emphasis"
	case "underscores":
		return "Underscore emphasis"
	case "headings":
		return "Headings"
	case "blockquotes":
		return "Block quotes"
	case "lists":
		return "Lists"
	case "code":
		return "Code"
	case "strikethrough":
		return "Strikethrough"
	case "rules":
		return "Thematic breaks"
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
	case "asterisks":
		return "Type * around text to emphasize it, ** to make it strong. {markup.emphasis} and {markup.strong} do too."
	case "underscores":
		return "Type _ and __ the same way. Off, snake_case stays as typed."
	case "headings":
		return "Type one to six # and a space to start a heading."
	case "blockquotes":
		return "Type > and a space to start a quote."
	case "lists":
		return "Type - or + and a space for a bullet, a number and a period for a numbered item."
	case "code":
		return "Type ` around text for code, ``` and Return for a code block."
	case "strikethrough":
		return "Type ~ around text to strike it through."
	case "rules":
		return "Type --- on a line of its own for a thematic break."
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
