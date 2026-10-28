package editor

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

func TestRustOracleMarkupProjection(t *testing.T) {
	data, err := os.ReadFile("../../fixtures/rust-oracle/v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name           string `json:"fixture"`
		SemanticText   string `json:"semantic_text"`
		ProjectedText  string `json:"projected_text"`
		SourceRevision uint64 `json:"source_revision"`
		Generation     uint64 `json:"generation"`
		Segments       []struct {
			Start int     `json:"start"`
			End   int     `json:"end"`
			Kind  string  `json:"kind"`
			Key   *string `json:"key"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	var oracle *struct {
		Name           string `json:"fixture"`
		SemanticText   string `json:"semantic_text"`
		ProjectedText  string `json:"projected_text"`
		SourceRevision uint64 `json:"source_revision"`
		Generation     uint64 `json:"generation"`
		Segments       []struct {
			Start int     `json:"start"`
			End   int     `json:"end"`
			Kind  string  `json:"kind"`
			Key   *string `json:"key"`
		} `json:"segments"`
	}
	for index := range fixtures {
		if fixtures[index].Name == "markup-projection-v1" {
			oracle = &fixtures[index]
			break
		}
	}
	if oracle == nil {
		t.Fatal("markup-projection-v1 fixture missing")
	}
	document, err := model.NewDocument("text")
	if err != nil {
		t.Fatal(err)
	}
	node := document.Snapshot().Root()
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		if err := draft.SetTextValue(node, oracle.SemanticText); err != nil {
			return err
		}
		if _, err := draft.AddMark(node, model.NewUTF16Range(0, 5), 1); err != nil {
			return err
		}
		_, err := draft.AddMark(node, model.NewUTF16Range(6, 10), 2)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	catalog, err := EditorPolicyCatalog()
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := kernel.NewPolicyRuntime(document, catalog)
	if err != nil {
		t.Fatal(err)
	}
	projected, err := MarkupProject(runtime.Snapshot(), node, oracle.Generation, runtime.Configuration())
	if err != nil {
		t.Fatal(err)
	}
	if projected.Text() != oracle.ProjectedText {
		t.Fatalf("projected text = %q, want %q", projected.Text(), oracle.ProjectedText)
	}
	if projected.Version().SourceRevision() != oracle.SourceRevision ||
		projected.Version().Generation() != oracle.Generation {
		t.Fatalf("version = %#v", projected.Version())
	}
	segments := projected.Segments()
	if len(segments) != len(oracle.Segments) {
		t.Fatalf("segments = %#v, want %#v", segments, oracle.Segments)
	}
	for index, segment := range segments {
		want := oracle.Segments[index]
		if int(segment.ProjectedRange().Start) != want.Start ||
			int(segment.ProjectedRange().End) != want.End {
			t.Fatalf("segment %d range = %#v, want %#v", index, segment.ProjectedRange(), want)
		}
		switch want.Kind {
		case "attribute":
			target, ok := segment.AttributeTarget()
			if !ok || want.Key == nil || target.Key() != *want.Key {
				t.Fatalf("segment %d attribute = %#v, want %#v", index, target, want)
			}
		case "editable":
			target, ok := segment.SemanticTarget()
			if !ok || target.Node() != node {
				t.Fatalf("segment %d editable = %#v", index, target)
			}
		default:
			_, hasSemantic := segment.SemanticTarget()
			_, hasAttribute := segment.AttributeTarget()
			if hasSemantic || hasAttribute {
				t.Fatalf("segment %d generated source = %#v", index, segment)
			}
		}
	}
}
