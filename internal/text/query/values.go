package query

import (
	"strconv"
	"strings"
	"time"
)

// Dates is a span of time, From inclusive and To exclusive; a zero end
// is open.
type Dates struct{ From, To time.Time }

// Contains reports whether t falls in the span.
func (d Dates) Contains(t time.Time) bool {
	return (d.From.IsZero() || !t.Before(d.From)) && (d.To.IsZero() || t.Before(d.To))
}

// ParseDates reads a date value at now, in now's location:
//
//   - a day, month or year: 2026-11-14, 2026-11, 2026
//   - a named one: today, yesterday, this-week, last-week, this-month,
//     last-month, this-year, last-year (weeks start on Monday)
//   - an age: 3d, 2w, 6m, 1y, meaning within that long before now
//   - a comparison: >X after X, >=X from X on, <X before X, <=X up to the
//     end of X; for an age, < is newer and > older, as <3d is within 3
//     days and >3d longer ago
//   - a span: X..Y from X to the end of Y, either end left open
func ParseDates(v string, now time.Time) (Dates, bool) {
	v = Fold(strings.TrimSpace(v))
	if a, b, ok := strings.Cut(v, ".."); ok {
		var d Dates
		if a != "" {
			p, ok := datePoint(a, now)
			if !ok {
				return Dates{}, false
			}
			d.From = p.from
		}
		if b != "" {
			p, ok := datePoint(b, now)
			if !ok {
				return Dates{}, false
			}
			d.To = p.to
		}
		return d, a != "" || b != ""
	}
	cmp := ""
	for _, c := range []string{">=", "<=", ">", "<"} {
		if strings.HasPrefix(v, c) {
			cmp, v = c, v[len(c):]
			break
		}
	}
	p, ok := datePoint(v, now)
	if !ok {
		return Dates{}, false
	}
	if p.age {
		// An age is an instant before now: < and bare are newer than it.
		switch cmp {
		case ">", ">=":
			return Dates{To: p.from}, true
		}
		return Dates{From: p.from}, true
	}
	switch cmp {
	case ">":
		return Dates{From: p.to}, true
	case ">=":
		return Dates{From: p.from}, true
	case "<":
		return Dates{To: p.from}, true
	case "<=":
		return Dates{To: p.to}, true
	}
	return Dates{From: p.from, To: p.to}, true
}

// parseDates checks a date value's spelling.
func parseDates(v string) (Dates, bool) {
	return ParseDates(v, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
}

// point is a date value's period; an age's is the instant it names, from
// and to alike.
type point struct {
	from, to time.Time
	age      bool
}

func datePoint(v string, now time.Time) (point, bool) {
	loc := now.Location()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	week := day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	year := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, loc)
	span := func(from, to time.Time) (point, bool) { return point{from: from, to: to}, true }
	switch v {
	case "today":
		return span(day, day.AddDate(0, 0, 1))
	case "yesterday":
		return span(day.AddDate(0, 0, -1), day)
	case "this-week":
		return span(week, week.AddDate(0, 0, 7))
	case "last-week":
		return span(week.AddDate(0, 0, -7), week)
	case "this-month":
		return span(month, month.AddDate(0, 1, 0))
	case "last-month":
		return span(month.AddDate(0, -1, 0), month)
	case "this-year":
		return span(year, year.AddDate(1, 0, 0))
	case "last-year":
		return span(year.AddDate(-1, 0, 0), year)
	}
	if n := len(v); n >= 2 {
		if count, err := strconv.Atoi(v[:n-1]); err == nil && count >= 0 {
			at, unit := now, true
			switch v[n-1] {
			case 'd':
				at = now.AddDate(0, 0, -count)
			case 'w':
				at = now.AddDate(0, 0, -7*count)
			case 'm':
				at = now.AddDate(0, -count, 0)
			case 'y':
				at = now.AddDate(-count, 0, 0)
			default:
				unit = false
			}
			if unit {
				return point{from: at, to: at, age: true}, true
			}
		}
	}
	for _, layout := range []struct {
		form    string
		y, m, d int
	}{{"2006-01-02", 0, 0, 1}, {"2006-01", 0, 1, 0}, {"2006", 1, 0, 0}} {
		if len(v) != len(layout.form) {
			continue
		}
		t, err := time.ParseInLocation(layout.form, v, loc)
		if err != nil {
			continue
		}
		return span(t, t.AddDate(layout.y, layout.m, layout.d))
	}
	return point{}, false
}

// Counts is a span of counts, both ends inclusive; an end not had is open.
type Counts struct {
	Min, Max       int
	HasMin, HasMax bool
}

// Contains reports whether n falls in the span.
func (c Counts) Contains(n int) bool {
	return (!c.HasMin || n >= c.Min) && (!c.HasMax || n <= c.Max)
}

// ParseCounts reads a count value: 500 (at least 500), >500, >=500, <100,
// <=100, 100..500, either end of a span left open; k counts thousands, as
// 2k.
func ParseCounts(v string) (Counts, bool) {
	v = Fold(strings.TrimSpace(v))
	num := func(s string) (int, bool) {
		mul := 1
		if strings.HasSuffix(s, "k") {
			mul, s = 1000, s[:len(s)-1]
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil && f >= 0 {
			return int(f * float64(mul)), true
		}
		return 0, false
	}
	if a, b, ok := strings.Cut(v, ".."); ok {
		var c Counts
		var good bool
		if a != "" {
			if c.Min, good = num(a); !good {
				return Counts{}, false
			}
			c.HasMin = true
		}
		if b != "" {
			if c.Max, good = num(b); !good {
				return Counts{}, false
			}
			c.HasMax = true
		}
		return c, c.HasMin || c.HasMax
	}
	for _, cmp := range []string{">=", "<=", ">", "<"} {
		if rest, ok := strings.CutPrefix(v, cmp); ok {
			n, ok := num(rest)
			if !ok {
				return Counts{}, false
			}
			switch cmp {
			case ">=":
				return Counts{Min: n, HasMin: true}, true
			case ">":
				return Counts{Min: n + 1, HasMin: true}, true
			case "<=":
				return Counts{Max: n, HasMax: true}, true
			default:
				return Counts{Max: n - 1, HasMax: true}, true
			}
		}
	}
	n, ok := num(v)
	return Counts{Min: n, HasMin: true}, ok
}
