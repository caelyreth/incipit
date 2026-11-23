package query

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
	Name string
	// Alias is the key this one is another name for. The key's words,
	// its Label, Help and sentences, are the catalog's (words.go).
	Alias string
	Takes Takes
	// Alone is a key of words that may be written bare, as $todo, or with
	// them, as $todo:budget.
	Alone   bool
	Scope   Scope
	Choices []string
	// Setting is a key that sets how the query runs, not a condition a
	// sheet meets: what it says holds of the whole query, so - cannot turn
	// it.
	Setting bool
}

// Kinds are what $has looks for.
var Kinds = []string{"code", "image", "link", "list", "math", "quote", "table", "task"}

// SortFields are what $sort orders by; a leading "-" turns the order.
var SortFields = []string{"relevance", "updated", "created", "title", "words"}

// Lenses are what $bands stands the field by (field-and-desk.md §4.7.5):
// each a way of saying what is near, none an order.
var Lenses = []string{"activity", "relevance", "updated", "created", "tag"}

// Keys are the query language's keys, in the order completion offers them.
var Keys = []Key{
	{Name: "tag", Takes: TakesName, Scope: InNotebook},
	{Name: "in", Takes: TakesName, Scope: InNotebook},
	{Name: "updated", Takes: TakesDate, Scope: InNotebook},
	{Name: "date", Alias: "updated", Takes: TakesDate, Scope: InNotebook},
	{Name: "created", Takes: TakesDate, Scope: InNotebook},
	{Name: "words", Takes: TakesNumber, Scope: InNotebook},
	{Name: "title", Takes: TakesText, Scope: InNotebook},
	{Name: "heading", Takes: TakesText, Scope: Anywhere},
	{Name: "code", Takes: TakesText, Scope: Anywhere},
	{Name: "quote", Takes: TakesText, Scope: Anywhere},
	{Name: "has", Takes: TakesChoice, Scope: InNotebook, Choices: Kinds},
	{Name: "task", Takes: TakesText, Alone: true, Scope: InNotebook},
	{Name: "todo", Takes: TakesText, Alone: true, Scope: InNotebook},
	{Name: "done", Takes: TakesText, Alone: true, Scope: InNotebook},
	{Name: "link", Takes: TakesName, Scope: InNotebook},
	{Name: "inbox", Takes: TakesNothing, Scope: InNotebook},
	{Name: "trash", Takes: TakesNothing, Scope: InNotebook, Setting: true},
	{Name: "open", Takes: TakesNothing, Scope: InNotebook},
	{Name: "settings", Takes: TakesNothing, Scope: InNotebook, Setting: true},
	{Name: "sheet", Takes: TakesNothing, Scope: Anywhere, Setting: true},
	{Name: "sort", Takes: TakesChoice, Scope: InNotebook, Choices: SortFields, Setting: true},
	{Name: "bands", Takes: TakesChoice, Scope: InNotebook, Choices: Lenses, Setting: true},
	{Name: "case", Takes: TakesNothing, Scope: InSheet, Setting: true},
	{Name: "word", Takes: TakesNothing, Scope: InSheet, Setting: true},
}

// Bare reports whether the key may be written with no value: one that
// takes none, or one whose words may be left out (Alone).
func (k *Key) Bare() bool { return k.Takes == TakesNothing || k.Alone }

// OneValue reports a key a query holds one value of: a setting, a span of
// dates or of lengths. A value taken for it takes the place of the one
// there; others, as tags or kinds, gather.
func (k *Key) OneValue() bool {
	return k.Setting || k.Takes == TakesDate || k.Takes == TakesNumber
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
