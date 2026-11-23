package query

import (
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/i18n"
)

// Describe says a key's value in words, as a filter shows it: 7d as "in
// the last 7 days", <100 as "fewer than 100", -updated as "updated,
// oldest first", a kind as its name, a text value in quotes. It reads the
// value as the language does, so what it says is what the value finds. A
// value the key does not take is said as written.
func Describe(k *Key, v string) string {
	if !k.Accepts(v) || strings.TrimSpace(v) == "" {
		return v
	}
	switch k.Takes {
	case TakesText:
		return locales.Quoted(v)
	case TakesDate:
		return describeDates(Fold(strings.TrimSpace(v)))
	case TakesNumber:
		return describeCounts(Fold(strings.TrimSpace(v)))
	case TakesChoice:
		return describeChoice(k.Name, Fold(v))
	}
	return v
}

// datePresets are the named dates as CLDR says them: this one, or the one
// before, of a unit.
var datePresets = map[string]struct {
	ago  float64
	unit i18n.RelativeTimeUnit
}{
	"today": {0, i18n.RelativeDay}, "yesterday": {-1, i18n.RelativeDay},
	"this-week": {0, i18n.RelativeWeek}, "last-week": {-1, i18n.RelativeWeek},
	"this-month": {0, i18n.RelativeMonth}, "last-month": {-1, i18n.RelativeMonth},
	"this-year": {0, i18n.RelativeYear}, "last-year": {-1, i18n.RelativeYear},
}

// datePresetWords is the named date v in words, false if v names none.
func datePresetWords(v string) (string, bool) {
	p, ok := datePresets[v]
	if !ok {
		return "", false
	}
	return i18n.FormatRelativeTime(i18n.Locale(), p.ago, p.unit, i18n.RelativeTimeOptions{Auto: true}), true
}

func describeDates(v string) string {
	if a, b, ok := strings.Cut(v, ".."); ok {
		switch {
		case a == "":
			return locales.UpTo(datePointWords(b))
		case b == "":
			return locales.FromPoint(datePointWords(a))
		}
		return locales.FromTo(datePointWords(a), datePointWords(b))
	}
	cmp := ""
	for _, c := range []string{">=", "<=", ">", "<"} {
		if strings.HasPrefix(v, c) {
			cmp, v = c, v[len(c):]
			break
		}
	}
	if n, unit, ok := age(v); ok {
		if cmp == ">" || cmp == ">=" {
			return locales.MoreThanAgo(span(n, unit))
		}
		return map[byte]func(int) string{
			'd': locales.InLastDays, 'w': locales.InLastWeeks, 'm': locales.InLastMonths, 'y': locales.InLastYears,
		}[unit](n)
	}
	point := datePointWords(v)
	switch cmp {
	case ">":
		return locales.AfterPoint(point)
	case ">=":
		return locales.FromPoint(point)
	case "<":
		return locales.BeforePoint(point)
	case "<=":
		return locales.UpTo(point)
	}
	if _, named := datePresets[v]; named {
		return point
	}
	if len(v) == len("2006-01-02") {
		return locales.OnDay(point)
	}
	return locales.InPeriod(point)
}

// datePointWords is a day, month, year or named date in words.
func datePointWords(v string) string {
	if w, ok := datePresetWords(v); ok {
		return w
	}
	if n, unit, ok := age(v); ok {
		return locales.SpanAgo(span(n, unit))
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

// span is n of unit in words, as 3 days.
func span(n int, unit byte) string {
	return map[byte]func(int) string{
		'd': locales.SpanDays, 'w': locales.SpanWeeks, 'm': locales.SpanMonths, 'y': locales.SpanYears,
	}[unit](n)
}

func describeCounts(v string) string {
	if a, b, ok := strings.Cut(v, ".."); ok {
		switch {
		case a == "":
			return locales.OrFewer(b)
		case b == "":
			return locales.OrMore(a)
		}
		return locales.FromTo(a, b)
	}
	switch {
	case strings.HasPrefix(v, ">="):
		return locales.OrMore(v[2:])
	case strings.HasPrefix(v, "<="):
		return locales.OrFewer(v[2:])
	case strings.HasPrefix(v, ">"):
		return locales.MoreThan(v[1:])
	case strings.HasPrefix(v, "<"):
		return locales.FewerThan(v[1:])
	}
	return locales.OrMore(v)
}

// choiceWords are the choices of the keys that take them, in words.
var choiceWords = map[string]map[string]func() string{
	"has": {
		"code": locales.KindCode, "image": locales.KindImage, "link": locales.KindLink, "list": locales.KindList,
		"math": locales.KindMath, "quote": locales.KindQuote, "table": locales.KindTable, "task": locales.KindTask,
	},
	"sort": {
		"relevance": locales.SortRelevance, "updated": locales.SortUpdated, "created": locales.SortCreated,
		"title": locales.SortTitle, "words": locales.SortWords,
	},
	"bands": {
		"activity": locales.LensActivity, "relevance": locales.LensRelevance, "updated": locales.LensUpdated,
		"created": locales.LensCreated, "tag": locales.LensTag,
	},
}

// describeChoice says a choice of the key name; an order, which comes
// first in it, as the index sorts it (dates oldest first, lengths longest
// first, titles A to Z, a - turning it).
func describeChoice(name, v string) string {
	turned := name == "sort" && strings.HasPrefix(v, "-")
	v = strings.TrimPrefix(v, "-")
	w, ok := choiceWords[name][v]
	if !ok {
		return v
	}
	firsts := map[string][2]func() string{
		"updated": {locales.OldestFirst, locales.NewestFirst},
		"created": {locales.OldestFirst, locales.NewestFirst},
		"words":   {locales.LongestFirst, locales.ShortestFirst},
		"title":   {locales.AToZ, locales.ZToA},
	}
	f, ordered := firsts[v]
	if name != "sort" || !ordered {
		return w()
	}
	if turned {
		return locales.SortOrder(w(), f[1]())
	}
	return locales.SortOrder(w(), f[0]())
}
