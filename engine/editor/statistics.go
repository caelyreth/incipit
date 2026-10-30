package editor

import (
	"strings"

	"github.com/caelyreth/neverland/engine/model"
)

// TextMetrics are semantic counts over a page's text. Characters
// means Unicode scalars, not UTF-16 units or graphemes.
type TextMetrics struct {
	Characters int
	Words      int
	Paragraphs int
}

// ComputeTextMetrics counts scalars, UAX #29 word-like segments, and nonblank
// lines separated only by '\n'.
func ComputeTextMetrics(text string) (TextMetrics, bool) {
	ranges, ok := WordLikeRanges(text)
	if !ok {
		return TextMetrics{}, false
	}
	return TextMetrics{
		Characters: model.StringLenRunes(text),
		Words:      len(ranges),
		Paragraphs: countParagraphs(text),
	}, true
}

func countParagraphs(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

// PageMetrics counts a page block by block. Each block keeps its counts for
// as long as its text root is the same, so an edit recounts only the blocks
// it touched, and the page's counts are their sum.
type PageMetrics struct {
	blocks map[model.NodeID]blockMetrics
	total  TextMetrics
}

type blockMetrics struct {
	root    model.TextRoot
	hasRoot bool
	metrics TextMetrics
}

// Update recounts what changed in document since the last update.
func (m *PageMetrics) Update(document model.DocumentSnapshot) bool {
	next := make(map[model.NodeID]blockMetrics, len(m.blocks))
	total := TextMetrics{}
	for _, block := range DocumentBlocks(document) {
		root, hasRoot := block.TextRoot()
		cached, ok := m.blocks[block.ID()]
		if !ok || cached.hasRoot != hasRoot || hasRoot && !cached.root.Same(root) {
			cached = blockMetrics{root: root, hasRoot: hasRoot}
			if hasRoot {
				metrics, valid := ComputeTextMetrics(root.String())
				if !valid {
					return false
				}
				cached.metrics = metrics
			}
		}
		next[block.ID()] = cached
		total.Characters += cached.metrics.Characters
		total.Words += cached.metrics.Words
		total.Paragraphs += cached.metrics.Paragraphs
	}
	m.blocks, m.total = next, total
	return true
}

// Total returns the page's counts as last updated.
func (m *PageMetrics) Total() TextMetrics { return m.total }
