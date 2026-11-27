package search

import (
	"github.com/caelyreth/incipit/internal/text/query"

	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/profile"
)

// Content is what the index reads of a page's own text: its texts of each
// kind, one block a line, the kinds of thing it holds and the sites its
// text names. A page's title, tags and words are its summary's
// (storage.Summarize), never read twice.
type Content struct {
	Headings, Body, Code, Quote string
	Kinds                       []string
	Domains                     []string
	// Tasks are its task items, in order.
	Tasks []Task
}

// Read reads a page's content. Body holds every block's text, the other
// texts' too: code, quotes, captions and cells. Formulas are read as the
// kind they are, not as their source.
func Read(root *doc.Node) Content {
	r := reader{kinds: map[string]bool{}, domains: map[string]bool{}}
	r.node(root, false)
	c := Content{
		Headings: strings.Join(r.headings, "\n"),
		Body:     strings.Join(r.body, "\n"),
		Code:     strings.Join(r.code, "\n"),
		Quote:    strings.Join(r.quote, "\n"),
	}
	for k := range r.kinds {
		c.Kinds = append(c.Kinds, k)
	}
	for d := range r.domains {
		c.Domains = append(c.Domains, d)
	}
	slices.Sort(c.Kinds)
	slices.Sort(c.Domains)
	c.Tasks = r.tasks
	return c
}

type reader struct {
	headings, body, code, quote []string
	kinds, domains              map[string]bool
	tasks                       []Task
}

// kindNames are the kinds Read reports a node of.
var kindNames = map[doc.Kind]string{
	profile.Code: "code", profile.Figure: "image", profile.List: "list",
	profile.Math: "math", profile.Formula: "math", profile.Quote: "quote", profile.Table: "table",
}

func (r *reader) node(n *doc.Node, quoted bool) {
	if name, ok := kindNames[n.Kind()]; ok {
		r.kinds[name] = true
	}
	if n.Kind() == profile.Math {
		return
	}
	// A task is its item's own line, what is nested in it apart.
	if task := profile.TaskOf(n); n.Kind() == profile.Item && task != profile.NoTask {
		r.kinds["task"] = true
		text := strings.Join(strings.Fields(n.Slot(0).Content().Text()), " ")
		r.tasks = append(r.tasks, Task{ID: strconv.FormatUint(uint64(n.ID()), 10), Text: text, Done: task == profile.TaskDone})
	}
	quoted = quoted || n.Kind() == profile.Quote
	for _, slot := range n.Slots() {
		if !slot.IsInline() {
			for _, c := range slot.Children() {
				r.node(c, quoted)
			}
			continue
		}
		r.inline(n, slot.Content(), quoted)
	}
}

// inline reads one inline slot of n.
func (r *reader) inline(n *doc.Node, in *doc.Inline, quoted bool) {
	var text, code strings.Builder
	in.Each(func(_ int, it doc.Item) bool {
		if it.Node != nil {
			if name, ok := kindNames[it.Node.Kind()]; ok {
				r.kinds[name] = true
			}
			text.WriteByte(' ')
			return true
		}
		text.WriteString(it.Text)
		if it.Marks.Has(profile.CodeMark) {
			code.WriteString(it.Text + " ")
			r.kinds["code"] = true
		}
		return true
	})
	t := strings.TrimSpace(text.String())
	if t == "" {
		return
	}
	r.body = append(r.body, t)
	switch {
	case n.Kind() == profile.Heading:
		r.headings = append(r.headings, t)
	case n.Kind() == profile.Code:
		r.code = append(r.code, t)
	}
	if c := strings.TrimSpace(code.String()); c != "" {
		r.code = append(r.code, c)
	}
	if quoted {
		r.quote = append(r.quote, t)
	}
	if n.Kind() != profile.Code {
		for _, d := range Domains(t) {
			r.domains[d] = true
			r.kinds["link"] = true
		}
	}
}

// Domains are the sites the web addresses written in a text name, as
// https://docs.github.com/x or www.example.com do: the host, folded,
// without www.
func Domains(text string) []string {
	var out []string
	for i := 0; i < len(text); {
		host := ""
		switch {
		case hasPrefixFold(text[i:], "www.") && (i == 0 || !wordRune(text, i)):
			host = hostAt(text, i)
		case strings.HasPrefix(text[i:], "://"):
			j := i
			for j > 0 && isScheme(text[j-1]) {
				j--
			}
			if j < i && (j == 0 || !wordRune(text, j)) {
				host = hostAt(text, i+3)
			}
		}
		if host != "" {
			out = append(out, strings.TrimPrefix(host, "www."))
			i += len(host)
			continue
		}
		i++
	}
	return out
}

// hostAt is the host written at i: letters, digits, dots and hyphens up to
// a port, a path or the address's end, with at least one dot inside.
func hostAt(text string, i int) string {
	j := i
	for j < len(text) {
		r, n := utf8.DecodeRuneInString(text[j:])
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-') || query.Unspaced(r) {
			break
		}
		j += n
	}
	host := strings.Trim(text[i:j], ".-")
	if !strings.Contains(host, ".") {
		return ""
	}
	return query.Fold(host)
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

func isScheme(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '+' || b == '-' || b == '.'
}

// wordRune reports whether the character before i is a letter or digit.
func wordRune(text string, i int) bool {
	r, _ := utf8.DecodeLastRuneInString(text[:i])
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
