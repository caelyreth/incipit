package surface

import (
	"fmt"
	"strings"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// Join is the flow of parts one after another, a line break between each
// and the next in the style breaks gives it, as a page is its blocks each
// prepared alone: each part's runs, blocks and objects move with it. Parts carry no tables, which span the
// blocks of a page and are its to say.
func Join(version projection.Version, parts []*Flow, breaks []TextStyle) (*Flow, error) {
	if len(parts) == 0 || len(breaks) != len(parts)-1 {
		return nil, fmt.Errorf("a join takes a part at least and a break between each and the next")
	}
	size, count, objects := 0, 0, 0
	for _, part := range parts {
		if len(part.tables) > 0 {
			return nil, fmt.Errorf("a joined part has no tables")
		}
		size += len(part.text) + 1
		count += len(part.blocks)
		objects += len(part.objects)
	}
	var text strings.Builder
	text.Grow(size)
	runs := make([]StyleRun, 0, count+len(breaks))
	blocks := make([]Block, 0, count)
	moved := make([]Object, 0, objects)
	push := func(run StyleRun) {
		if last := len(runs) - 1; last >= 0 && runs[last].Style == run.Style && runs[last].Bytes.End == run.Bytes.Start {
			runs[last].Bytes.End = run.Bytes.End
			return
		}
		runs = append(runs, run)
	}
	at, at16 := 0, model.UTF16Offset(0)
	for index, part := range parts {
		if index > 0 {
			text.WriteByte('\n')
			push(StyleRun{Bytes: model.NewByteRange(model.ByteOffset(at), model.ByteOffset(at+1)), Style: breaks[index-1]})
			at, at16 = at+1, at16+1
		}
		text.WriteString(part.text)
		for _, run := range part.runs {
			run.Bytes = model.NewByteRange(run.Bytes.Start+model.ByteOffset(at), run.Bytes.End+model.ByteOffset(at))
			push(run)
		}
		for _, block := range part.blocks {
			block.Range = model.NewUTF16Range(block.Range.Start+at16, block.Range.End+at16)
			blocks = append(blocks, block)
		}
		for _, object := range part.objects {
			moved = append(moved, object.moved(at16))
		}
		at, at16 = at+len(part.text), at16+part.UTF16Len()
	}
	flow, err := NewFlow(version, text.String(), runs, blocks)
	if err != nil {
		return nil, err
	}
	// Each part's objects stood in its own blocks, which stand as they
	// did, so they stand in the join as they may.
	flow.objects = moved
	return flow, nil
}
