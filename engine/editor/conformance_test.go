package editor

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

func TestConformanceMarkupProjection(t *testing.T) {
	data, err := os.ReadFile("../../fixtures/conformance/v1.json")
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
	var expected *struct {
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
			expected = &fixtures[index]
			break
		}
	}
	if expected == nil {
		t.Fatal("markup-projection-v1 fixture missing")
	}
	// The page and its block come in one transaction, as the fixture's node
	// was made in, so revisions line up.
	document, err := model.NewDocument(DocumentNode)
	if err != nil {
		t.Fatal(err)
	}
	var node model.NodeID
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		node, err = draft.CreateNode(document.Snapshot().Root(), 0, ParagraphNode)
		if err != nil {
			return err
		}
		if err := draft.SetTextValue(node, expected.SemanticText); err != nil {
			return err
		}
		if _, err := draft.AddMark(node, model.NewUTF16Range(0, 5), EmphasisMark, 1); err != nil {
			return err
		}
		_, err = draft.AddMark(node, model.NewUTF16Range(6, 10), EmphasisMark, 2)
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
	projected, err := ProjectSnapshot(runtime.Snapshot(), expected.Generation, runtime.Configuration())
	if err != nil {
		t.Fatal(err)
	}
	if projected.Text() != expected.ProjectedText {
		t.Fatalf("projected text = %q, want %q", projected.Text(), expected.ProjectedText)
	}
	if projected.Version().SourceRevision() != expected.SourceRevision ||
		projected.Version().Generation() != expected.Generation {
		t.Fatalf("version = %#v", projected.Version())
	}
	segments := projected.Segments()
	if len(segments) != len(expected.Segments) {
		t.Fatalf("segments = %#v, want %#v", segments, expected.Segments)
	}
	for index, segment := range segments {
		want := expected.Segments[index]
		if int(segment.ProjectedRange().Start) != want.Start ||
			int(segment.ProjectedRange().End) != want.End {
			t.Fatalf("segment %d range = %#v, want %#v", index, segment.ProjectedRange(), want)
		}
		switch want.Kind {
		case "attribute":
			// The fixture keys emphasis notation "emphasis/<id>"; here every
			// mark's notation is keyed by the mark alone.
			target, ok := segment.AttributeTarget()
			if !ok || want.Key == nil ||
				target.Key() != strings.Replace(*want.Key, "emphasis/", markerKeyPrefix, 1) {
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
