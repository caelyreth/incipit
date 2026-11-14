package search

// Scope is where a key means something: across the notebook, in the sheet
// being found in, or both.
type Scope uint8

const (
	InNotebook Scope = 1 << iota
	InSheet
	Anywhere = InNotebook | InSheet
)

// Takes is what a key's value is.
type Takes uint8

const (
	// TakesNothing is a key that stands alone, as $inbox.
	TakesNothing Takes = iota
	// TakesText is words or a phrase, as $title:"weekly notes".
	TakesText
	// TakesOptionalText is a key alone or with text, as $todo or
	// $todo:budget.
	TakesOptionalText
	// TakesName is a name: a tag, a group, a domain, a kind.
	TakesName
	// TakesDate is a date or a span of dates (ParseDates).
	TakesDate
	// TakesNumber is a count or a span of counts (ParseCounts).
	TakesNumber
	// TakesChoice is one of the key's Choices.
	TakesChoice
)

// Key is one $ key of the query language.
type Key struct {
	Name    string
	Takes   Takes
	Scope   Scope
	Choices []string
	// Help says what the key finds, for completion.
	Help string
}

// Kinds are what $has looks for.
var Kinds = []string{"code", "image", "link", "list", "math", "quote", "table", "task"}

// SortFields are what $sort orders by; a leading "-" turns the order.
var SortFields = []string{"relevance", "updated", "created", "title", "words"}

// Keys are the query language's keys, in the order completion offers them.
var Keys = []Key{
	{Name: "tag", Takes: TakesName, Scope: InNotebook, Help: "Sheets with a tag; $tag:none for sheets with none"},
	{Name: "in", Takes: TakesName, Scope: InNotebook, Help: "Sheets in a group or the groups inside it"},
	{Name: "updated", Takes: TakesDate, Scope: InNotebook, Help: "Sheets changed then: today, 7d, 2026-11, >2026-01-01"},
	{Name: "date", Takes: TakesDate, Scope: InNotebook, Help: "Same as $updated"},
	{Name: "created", Takes: TakesDate, Scope: InNotebook, Help: "Sheets made then"},
	{Name: "words", Takes: TakesNumber, Scope: InNotebook, Help: "Sheets of a length: 500 (at least), <100, 100..500"},
	{Name: "title", Takes: TakesText, Scope: InNotebook, Help: "Words in the title"},
	{Name: "heading", Takes: TakesText, Scope: Anywhere, Help: "Words in a heading"},
	{Name: "code", Takes: TakesText, Scope: Anywhere, Help: "Words in code"},
	{Name: "quote", Takes: TakesText, Scope: Anywhere, Help: "Words in a quote"},
	{Name: "has", Takes: TakesChoice, Scope: InNotebook, Choices: Kinds, Help: "Sheets holding a kind of thing"},
	{Name: "task", Takes: TakesOptionalText, Scope: Anywhere, Help: "A task, or a task with these words"},
	{Name: "todo", Takes: TakesOptionalText, Scope: Anywhere, Help: "An open task, or one with these words"},
	{Name: "done", Takes: TakesOptionalText, Scope: Anywhere, Help: "A done task, or one with these words"},
	{Name: "link", Takes: TakesName, Scope: Anywhere, Help: "Links to a site, as $link:github.com"},
	{Name: "inbox", Takes: TakesNothing, Scope: InNotebook, Help: "Sheets in the Inbox"},
	{Name: "trash", Takes: TakesNothing, Scope: InNotebook, Help: "Sheets in the trash"},
	{Name: "open", Takes: TakesNothing, Scope: InNotebook, Help: "Sheets open in a tab"},
	{Name: "sheet", Takes: TakesOptionalText, Scope: Anywhere, Help: "Find in this sheet, or in the sheet titled so"},
	{Name: "sort", Takes: TakesChoice, Scope: InNotebook, Choices: SortFields, Help: "Order: relevance, updated, created, title, words; -updated turns it"},
	{Name: "case", Takes: TakesNothing, Scope: InSheet, Help: "Match letter case"},
	{Name: "word", Takes: TakesNothing, Scope: InSheet, Help: "Match whole words only"},
}

// KeyNamed is the key called name, folded.
func KeyNamed(name string) (*Key, bool) {
	name = Fold(name)
	for i := range Keys {
		if Keys[i].Name == name {
			return &Keys[i], true
		}
	}
	return nil, false
}
