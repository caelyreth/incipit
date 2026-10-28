package editor

import (
	"strings"

	"github.com/caelyreth/neverland/engine/model"
)

// TextMetrics are semantic counts over the current node's text. Characters
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

// TextMetricsSource identifies the exact immutable root a metrics snapshot
// describes. It lets asynchronous work install only against current text.
type TextMetricsSource struct {
	Node    model.NodeID
	Root    model.TextRoot
	HasRoot bool
	Metrics TextMetrics
}

// NewTextMetricsSource captures a node text root and its computed metrics.
func NewTextMetricsSource(node model.NodeID, root model.TextRoot, hasRoot bool) (TextMetricsSource, bool) {
	metrics := TextMetrics{}
	if hasRoot {
		var ok bool
		metrics, ok = ComputeTextMetrics(root.String())
		if !ok {
			return TextMetricsSource{}, false
		}
	}
	return TextMetricsSource{Node: node, Root: root, HasRoot: hasRoot, Metrics: metrics}, true
}

// Matches reports whether this source still names the node's current text.
func (s TextMetricsSource) Matches(node *model.Node) bool {
	if node == nil || node.ID() != s.Node {
		return false
	}
	root, hasRoot := node.TextRoot()
	if hasRoot != s.HasRoot {
		return false
	}
	return !s.HasRoot || root.Same(s.Root)
}
