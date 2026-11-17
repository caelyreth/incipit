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
	// Label names the key in words, as a filter shows it, and Is and
	// IsNot join it to a value said in words (Describe), as in "Tag is
	// work" and "Content has no image". Alias is the key this one is
	// another name for.
	Label, Is, IsNot string
	Alias            string
	Takes            Takes
	Scope            Scope
	Choices          []string
	// Setting is a key that sets how the query runs, not a condition a
	// sheet meets: what it says holds of the whole query, so - cannot turn
	// it.
	Setting bool
	// Help says what the key finds, for completion.
	Help string
}

// Kinds are what $has looks for.
var Kinds = []string{"code", "image", "link", "list", "math", "quote", "table"}

// SortFields are what $sort orders by; a leading "-" turns the order.
var SortFields = []string{"relevance", "updated", "created", "title", "words"}

// Keys are the query language's keys, in the order completion offers them.
var Keys = []Key{
	{Name: "tag", Label: "Tag", Is: "is", IsNot: "is not", Takes: TakesName, Scope: InNotebook, Help: "Sheets with a tag; $tag:none for sheets with none"},
	{Name: "in", Label: "Group", Is: "is", IsNot: "is not", Takes: TakesName, Scope: InNotebook, Help: "Sheets in a group or the groups inside it"},
	{Name: "updated", Label: "Updated", IsNot: "not", Takes: TakesDate, Scope: InNotebook, Help: "Sheets changed then: today, 7d, 2026-11, >2026-01-01"},
	{Name: "date", Label: "Updated", IsNot: "not", Alias: "updated", Takes: TakesDate, Scope: InNotebook, Help: "Same as $updated"},
	{Name: "created", Label: "Created", IsNot: "not", Takes: TakesDate, Scope: InNotebook, Help: "Sheets made then"},
	{Name: "words", Label: "Words", IsNot: "not", Takes: TakesNumber, Scope: InNotebook, Help: "Sheets of a length: 500 (at least), <100, 100..500"},
	{Name: "title", Label: "Title", Is: "has", IsNot: "doesn’t have", Takes: TakesText, Scope: InNotebook, Help: "Words in the title"},
	{Name: "heading", Label: "Heading", Is: "has", IsNot: "doesn’t have", Takes: TakesText, Scope: Anywhere, Help: "Words in a heading"},
	{Name: "code", Label: "Code", Is: "has", IsNot: "doesn’t have", Takes: TakesText, Scope: Anywhere, Help: "Words in code"},
	{Name: "quote", Label: "Quote", Is: "has", IsNot: "doesn’t have", Takes: TakesText, Scope: Anywhere, Help: "Words in a quote"},
	{Name: "has", Label: "Content", Is: "has", IsNot: "has no", Takes: TakesChoice, Scope: InNotebook, Choices: Kinds, Help: "Sheets holding a kind of thing"},
	{Name: "link", Label: "Links", Is: "to", IsNot: "not to", Takes: TakesName, Scope: InNotebook, Help: "Links to a site, as $link:github.com"},
	{Name: "inbox", Label: "In the Inbox", Takes: TakesNothing, Scope: InNotebook, Help: "Sheets in the Inbox"},
	{Name: "trash", Label: "In the trash", Takes: TakesNothing, Scope: InNotebook, Setting: true, Help: "Sheets in the trash"},
	{Name: "open", Label: "Open in a tab", Takes: TakesNothing, Scope: InNotebook, Help: "Sheets open in a tab"},
	{Name: "settings", Label: "Settings", Takes: TakesNothing, Scope: InNotebook, Setting: true, Help: "The app's settings, found by their words"},
	{Name: "sheet", Label: "In this sheet", Takes: TakesNothing, Scope: Anywhere, Setting: true, Help: "Find in the sheet in front, as ⌘F does"},
	{Name: "sort", Label: "Sorted by", Takes: TakesChoice, Scope: InNotebook, Choices: SortFields, Setting: true, Help: "Order: relevance, updated, created, title, words; -updated turns it"},
	{Name: "case", Label: "Match case", Takes: TakesNothing, Scope: InSheet, Setting: true, Help: "Match letter case"},
	{Name: "word", Label: "Whole words", Takes: TakesNothing, Scope: InSheet, Setting: true, Help: "Match whole words only"},
}

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
