package query

import (
	"strconv"
	"strings"
)

// Describe says a key's value in words, as a filter shows it: 7d as "in
// the last 7 days", <100 as "fewer than 100", -updated as "updated,
// oldest first", a text value in quotes. It reads the value as the
// language does, so what it says is what the value finds. A value the key
// does not take is said as written.
func Describe(k *Key, v string) string {
	if !k.Accepts(v) || strings.TrimSpace(v) == "" {
		return v
	}
	switch k.Takes {
	case TakesText:
		return "“" + v + "”"
	case TakesDate:
		return describeDates(Fold(strings.TrimSpace(v)))
	case TakesNumber:
		return describeCounts(Fold(strings.TrimSpace(v)))
	case TakesChoice:
		if k.Name == "sort" {
			return describeSort(Fold(v))
		}
	}
	return v
}

// datePresetWords are the named dates in words.
var datePresetWords = map[string]string{
	"today": "today", "yesterday": "yesterday",
	"this-week": "this week", "last-week": "last week",
	"this-month": "this month", "last-month": "last month",
	"this-year": "this year", "last-year": "last year",
}

func describeDates(v string) string {
	if a, b, ok := strings.Cut(v, ".."); ok {
		switch {
		case a == "":
			return "up to " + datePointWords(b)
		case b == "":
			return "from " + datePointWords(a)
		}
		return datePointWords(a) + " to " + datePointWords(b)
	}
	cmp := ""
	for _, c := range []string{">=", "<=", ">", "<"} {
		if strings.HasPrefix(v, c) {
			cmp, v = c, v[len(c):]
			break
		}
	}
	if n, unit, ok := age(v); ok {
		span := unitWords(n, unit)
		if cmp == ">" || cmp == ">=" {
			return "more than " + span + " ago"
		}
		if n == 1 {
			return "in the last " + strings.TrimPrefix(span, "1 ")
		}
		return "in the last " + span
	}
	point := datePointWords(v)
	switch cmp {
	case ">":
		return "after " + point
	case ">=":
		return "from " + point
	case "<":
		return "before " + point
	case "<=":
		return "up to " + point
	}
	if _, named := datePresetWords[v]; named {
		return point
	}
	if len(v) == len("2006-01-02") {
		return "on " + point
	}
	return "in " + point
}

// datePointWords is a day, month, year or named date in words.
func datePointWords(v string) string {
	if w, ok := datePresetWords[v]; ok {
		return w
	}
	if n, unit, ok := age(v); ok {
		return unitWords(n, unit) + " ago"
	}
	return v
}

// age reads an age: a count and its unit, as 3d.
func age(v string) (int, byte, bool) {
	if len(v) < 2 || !strings.ContainsRune("dwmy", rune(v[len(v)-1])) {
		return 0, 0, false
	}
	n, err := strconv.Atoi(v[:len(v)-1])
	return n, v[len(v)-1], err == nil && n >= 0
}

func unitWords(n int, unit byte) string {
	word := map[byte]string{'d': "day", 'w': "week", 'm': "month", 'y': "year"}[unit]
	if n != 1 {
		word += "s"
	}
	return strconv.Itoa(n) + " " + word
}

func describeCounts(v string) string {
	if a, b, ok := strings.Cut(v, ".."); ok {
		switch {
		case a == "":
			return b + " or fewer"
		case b == "":
			return a + " or more"
		}
		return a + " to " + b
	}
	switch {
	case strings.HasPrefix(v, ">="):
		return v[2:] + " or more"
	case strings.HasPrefix(v, "<="):
		return v[2:] + " or fewer"
	case strings.HasPrefix(v, ">"):
		return "more than " + v[1:]
	case strings.HasPrefix(v, "<"):
		return "fewer than " + v[1:]
	}
	return v + " or more"
}

// describeSort says an order: what it goes by, and which come first, as
// the index sorts it (dates oldest first, lengths longest first, titles A
// to Z, a - turning it).
func describeSort(v string) string {
	turned := strings.HasPrefix(v, "-")
	v = strings.TrimPrefix(v, "-")
	firsts := map[string][2]string{
		"updated": {"oldest first", "newest first"},
		"created": {"oldest first", "newest first"},
		"words":   {"longest first", "shortest first"},
		"title":   {"A to Z", "Z to A"},
	}
	f, ok := firsts[v]
	if !ok {
		return v
	}
	if turned {
		return v + ", " + f[1]
	}
	return v + ", " + f[0]
}

// Sentence says a condition whole, in words: "Tag is not work", "Updated
// in the last 7 days", "Not in the Inbox".
func (c Condition) Sentence() string {
	k := c.Key
	not := c.Not && !k.Setting
	if k.Takes == TakesNothing {
		if not {
			return "Not " + strings.ToLower(k.Label[:1]) + k.Label[1:]
		}
		return k.Label
	}
	parts := []string{k.Label}
	verb := k.Is
	if not {
		verb = k.IsNot
	}
	if verb != "" {
		parts = append(parts, verb)
	}
	if strings.TrimSpace(c.Value) == "" {
		parts = append(parts, "…")
	} else {
		parts = append(parts, Describe(k, c.Value))
	}
	return strings.Join(parts, " ")
}
