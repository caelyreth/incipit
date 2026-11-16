package stage

import (
	"cmp"
	"math"
	"slices"
	"time"
)

// Signal holds the stable signals that determine an object's nearness to the
// pill.
type Signal struct {
	ActiveOnDesk  bool
	OnDesk        bool
	InActiveScope bool
	UpdatedAgo    time.Duration
	Words         int
}

// Nearness computes the deterministic score ordering items outward from the
// pill. Higher scores stand nearer the pill.
func Nearness(s Signal) float64 {
	var score float64
	if s.ActiveOnDesk {
		score += 1000
	}
	if s.OnDesk {
		score += 500
	}
	if s.InActiveScope {
		score += 120
	}
	// Smooth recency decay: hours/days matter strongly without needing a
	// separate "recent" slot; very old items flatten gently.
	hours := max(0, s.UpdatedAgo.Hours())
	score += 200.0 / (1.0 + math.Log1p(hours/6.0))
	if s.Words > 0 {
		score += min(30.0, math.Log1p(float64(s.Words))*2.5)
	}
	return score
}

// EmphasisStyle describes how a query alters a card visually without changing
// its position in the field.
type EmphasisStyle struct {
	Scale      float32
	Opacity    float32
	Brightness float32
	Depth      float32
}

// Emphasis computes visual prominence from query match relevance (0..1).
// When hasQuery is false, every card rests at neutral emphasis.
func Emphasis(hasQuery bool, match float32) EmphasisStyle {
	if !hasQuery {
		return EmphasisStyle{
			Scale:      1,
			Opacity:    1,
			Brightness: 1,
			Depth:      0,
		}
	}
	m := clamp01(match)
	if m <= 0 {
		return EmphasisStyle{
			Scale:      0.94,
			Opacity:    0.35,
			Brightness: 0.72,
			Depth:      -24,
		}
	}
	return EmphasisStyle{
		Scale:      1 + 0.05*m,
		Opacity:    1,
		Brightness: 1 + 0.14*m,
		Depth:      24 * m,
	}
}

// SortByNearness returns a sorted copy of items ordered from highest nearness
// to lowest, breaking ties deterministically by ID.
func SortByNearness(items []FieldItem) []FieldItem {
	out := slices.Clone(items)
	slices.SortStableFunc(out, func(a, b FieldItem) int {
		na, nb := Nearness(a.Signal), Nearness(b.Signal)
		if c := cmp.Compare(nb, na); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return out
}
