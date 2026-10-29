package editor

import (
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/text/rope"
)

// incrementalTextMetrics adjusts one installed snapshot for the common
// single-edit typing path. Operations touching more than one text range fall
// back to a full recount.
func incrementalTextMetrics(
	beforeRoot model.TextRoot,
	beforeHasRoot bool,
	afterRoot model.TextRoot,
	afterHasRoot bool,
	edits []model.TextEdit,
) (TextMetrics, bool) {
	if len(edits) != 1 {
		return TextMetrics{}, false
	}
	edit := edits[0]
	before := beforeRoot.Tree()
	if !beforeHasRoot {
		before = rope.Tree{}
	}
	after := afterRoot.Tree()
	if !afterHasRoot {
		after = rope.Tree{}
	}
	oldStart, ok := before.CheckedByteOfUTF16(int(edit.RangeUTF16().Start))
	if !ok {
		return TextMetrics{}, false
	}
	oldEnd, ok := before.CheckedByteOfUTF16(int(edit.RangeUTF16().End))
	if !ok {
		return TextMetrics{}, false
	}
	newStart, ok := after.CheckedByteOfUTF16(int(edit.RangeUTF16().Start))
	if !ok {
		return TextMetrics{}, false
	}
	startOffset := edit.RangeUTF16().Start
	inserted := edit.InsertedUTF16Len()
	maxOffset := model.UTF16Offset(int(^uint(0) >> 1))
	if startOffset < 0 || inserted < 0 ||
		startOffset > maxOffset-inserted {
		return TextMetrics{}, false
	}
	newEndOffset := startOffset + inserted
	newEnd, ok := after.CheckedByteOfUTF16(int(newEndOffset))
	if !ok {
		return TextMetrics{}, false
	}

	oldFrom, oldTo := paragraphBounds(before, oldStart, oldEnd)
	newFrom, newTo := paragraphBounds(after, newStart, newEnd)
	oldMetrics, ok := ComputeTextMetrics(before.Slice(oldFrom, oldTo))
	if !ok {
		return TextMetrics{}, false
	}
	newMetrics, ok := ComputeTextMetrics(after.Slice(newFrom, newTo))
	if !ok {
		return TextMetrics{}, false
	}
	return TextMetrics{
		Characters: newMetrics.Characters - oldMetrics.Characters,
		Words:      newMetrics.Words - oldMetrics.Words,
		Paragraphs: newMetrics.Paragraphs - oldMetrics.Paragraphs,
	}, true
}

func paragraphBounds(tree rope.Tree, start, end int) (int, int) {
	start = max(0, min(start, tree.ByteLen()))
	end = max(start, min(end, tree.ByteLen()))
	firstLine := tree.LineOf(start)
	lastLine := tree.LineOf(end)
	from := tree.LineStart(firstLine)
	to := tree.ByteLen()
	if lastLine+1 < tree.LineCount() {
		to = tree.LineStart(lastLine + 1)
	}
	return from, to
}
