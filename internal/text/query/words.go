package query

import (
	"strings"

	"github.com/caelyreth/incipit/locales"
)

// The query language is written the same in every language: $tag:work,
// $updated:7d, $sort:-updated. What the app says of it, a key's name,
// what it finds, a condition said whole, is the catalog's (package
// locales), in the language the app shows.

// keyWords are a key's words: its name, what it finds, and a condition of
// it said whole, of a value (is) or, for a key that may stand alone, of
// none (alone). A key that takes no value says itself whole in label;
// one that can be turned (- leaving out what it names) is told so.
type keyWords struct {
	label func() string
	help  func() string
	is    func(not, value string) string
	alone func(not string) string
	// turned is the label of a key that takes no value, turned.
	turned func(not string) string
}

var words = map[string]keyWords{
	"tag":      {label: locales.QueryTag, help: locales.QueryTagHelp, is: locales.QueryTagIs},
	"in":       {label: locales.QueryIn, help: locales.QueryInHelp, is: locales.QueryInIs},
	"updated":  {label: locales.QueryUpdated, help: locales.QueryUpdatedHelp, is: locales.QueryUpdatedIs},
	"date":     {label: locales.QueryUpdated, help: locales.QueryDateHelp, is: locales.QueryUpdatedIs},
	"created":  {label: locales.QueryCreated, help: locales.QueryCreatedHelp, is: locales.QueryCreatedIs},
	"words":    {label: locales.QueryWords, help: locales.QueryWordsHelp, is: locales.QueryWordsIs},
	"title":    {label: locales.QueryTitle, help: locales.QueryTitleHelp, is: locales.QueryTitleIs},
	"heading":  {label: locales.QueryHeading, help: locales.QueryHeadingHelp, is: locales.QueryHeadingIs},
	"code":     {label: locales.QueryCode, help: locales.QueryCodeHelp, is: locales.QueryCodeIs},
	"quote":    {label: locales.QueryQuote, help: locales.QueryQuoteHelp, is: locales.QueryQuoteIs},
	"has":      {label: locales.QueryHas, help: locales.QueryHasHelp, is: locales.QueryHasIs},
	"task":     {label: locales.QueryTask, help: locales.QueryTaskHelp, is: locales.QueryTaskIs, alone: locales.QueryTaskAlone},
	"todo":     {label: locales.QueryTodo, help: locales.QueryTodoHelp, is: locales.QueryTodoIs, alone: locales.QueryTodoAlone},
	"done":     {label: locales.QueryDone, help: locales.QueryDoneHelp, is: locales.QueryDoneIs, alone: locales.QueryDoneAlone},
	"link":     {label: locales.QueryLink, help: locales.QueryLinkHelp, is: locales.QueryLinkIs},
	"inbox":    {turned: locales.QueryInbox, help: locales.QueryInboxHelp},
	"trash":    {label: locales.QueryTrash, help: locales.QueryTrashHelp},
	"open":     {turned: locales.QueryOpen, help: locales.QueryOpenHelp},
	"settings": {label: locales.QuerySettings, help: locales.QuerySettingsHelp},
	"key":      {label: locales.QueryKey, help: locales.QueryKeyHelp, is: locales.QueryKeyIs},
	"sheet":    {label: locales.QuerySheet, help: locales.QuerySheetHelp},
	"sort":     {label: locales.QuerySort, help: locales.QuerySortHelp, is: func(_, v string) string { return locales.QuerySortIs(v) }},
	"bands":    {label: locales.QueryBands, help: locales.QueryBandsHelp, is: func(_, v string) string { return locales.QueryBandsIs(v) }},
	"case":     {label: locales.QueryCase, help: locales.QueryCaseHelp},
	"word":     {label: locales.QueryWord, help: locales.QueryWordHelp},
}

// Label names the key in words, as a filter shows it: Tag, Updated.
func (k *Key) Label() string {
	w := words[k.Name]
	if w.turned != nil {
		return w.turned("no")
	}
	return w.label()
}

// Help says what the key finds, for completion.
func (k *Key) Help() string { return words[k.Name].help() }

// Hint says what the key takes: words, a name, a date, a count, a kind;
// nothing for a key that stands alone.
func (k *Key) Hint() string {
	switch k.Takes {
	case TakesText:
		return locales.TakesWords()
	case TakesName:
		return locales.TakesName()
	case TakesDate:
		return locales.TakesDate()
	case TakesNumber:
		return locales.TakesCount()
	case TakesChoice:
		return locales.TakesKind()
	}
	return ""
}

// Prompt is what the key says before a value is written: "Title has…",
// "Updated…".
func (k *Key) Prompt() string {
	if k.Takes == TakesNothing {
		return k.Label()
	}
	return strings.TrimSpace(words[k.Name].is("no", "")) + locales.Pending()
}

// Sentence says a condition whole, in words: "Tag is not work", "Updated
// in the last 7 days", "Not in the Inbox".
func (c Condition) Sentence() string {
	k := c.Key
	w := words[k.Name]
	not := "no"
	if c.Not && !k.Setting {
		not = "yes"
	}
	switch {
	case k.Takes == TakesNothing && w.turned != nil:
		return w.turned(not)
	case k.Takes == TakesNothing:
		return w.label()
	case k.Alone && strings.TrimSpace(c.Value) == "":
		return w.alone(not)
	case strings.TrimSpace(c.Value) == "":
		return w.is(not, locales.Pending())
	}
	return w.is(not, Describe(k, c.Value))
}
