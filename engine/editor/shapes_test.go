package editor

import (
	"testing"
	"time"
)

// A block's modifier control offers prose shapes, its own marked, and each
// choice is one step of history that keeps the block's text and depth.
func TestShapeOptionsReshapeABlock(t *testing.T) {
	session := fixturePage(t, blockFixture{"nested item", blockShape{kind: BulletNode, depth: 1}, "item"})
	node := DocumentBlocks(session.Runtime().Snapshot().Document())[1].ID()
	options := session.ShapeOptions(node)
	var numbered, current *ShapeOption
	for i := range options {
		if options[i].Current {
			current = &options[i]
		}
		if options[i].Shape.Kind == OrderedNode {
			numbered = &options[i]
		}
	}
	if current == nil || current.Shape.Kind != BulletNode || numbered == nil || numbered.Shape.Depth != 1 {
		t.Fatalf("options = %#v", options)
	}
	if _, err := session.Do(Shape(node, numbered.Shape), time.Time{}); err != nil {
		t.Fatal(err)
	}
	block, _ := session.Runtime().Snapshot().Document().Node(node)
	if got := shapeOf(block); got.kind != OrderedNode || got.depth != 1 || block.Text().String() != "item" {
		t.Fatalf("reshaped to %#v %q", got, block.Text().String())
	}
	if _, err := session.doHistory(historyUndo); err != nil {
		t.Fatal(err)
	}
	block, _ = session.Runtime().Snapshot().Document().Node(node)
	if shapeOf(block).kind != BulletNode {
		t.Fatal("undo did not bring the bullet back")
	}
	code := fixturePage(t, blockFixture{"code", blockShape{kind: CodeNode}, "x"})
	if options := code.ShapeOptions(DocumentBlocks(code.Runtime().Snapshot().Document())[1].ID()); len(options) != 0 {
		t.Fatalf("code offers %#v", options)
	}
}
