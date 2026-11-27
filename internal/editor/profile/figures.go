package profile

import (
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// A figure shows a picture the notebook keeps, named by its media id, over
// a caption. The page never holds the picture's bytes: the figure records
// its media id and its size in pixels, so it lays out before the picture is
// read, and how it is set on the page.

// Picture is a kept picture: its media id and its size in pixels.
type Picture struct {
	Media         string
	Width, Height int
}

// PictureUse is what the host is asked to do with pictures.
type PictureUse uint8

const (
	// PictureInsert picks pictures to place where the selection is.
	PictureInsert PictureUse = iota
	// PictureCopy puts a figure's picture on the clipboard.
	PictureCopy
	// PictureSave saves a figure's picture out as a file.
	PictureSave
	// PicturePreview shows a figure's picture at its own size, apart from
	// the page.
	PicturePreview
	// PictureReplace picks a picture to show in a figure instead.
	PictureReplace
)

// PictureAsk is the data of the effect asking the host for pictures, or
// to use one: the figure and its picture, unless Use is PictureInsert.
type PictureAsk struct {
	Use    PictureUse
	Figure doc.ID
	Media  string
}

// SpanFull is the span of a figure across the whole column.
const SpanFull = 1000

// Shape is what a figure shows apart from its caption.
type Shape struct {
	Picture
	// Span is the share of the column the picture takes, in thousandths;
	// 0 sets it at its own size, at most the column.
	Span  int
	Align Align
}

// ShapeOf is a figure's shape.
func ShapeOf(n *doc.Node) Shape {
	s := Schema()
	return Shape{
		Picture: Picture{
			Media:  s.Attr(n, FieldMedia).Text(),
			Width:  int(s.Attr(n, FieldWidth).Int()),
			Height: int(s.Attr(n, FieldHeight).Int()),
		},
		Span:  int(s.Attr(n, FieldSpan).Int()),
		Align: Align(s.Attr(n, FieldAlign).Int()),
	}
}

// Attrs are the attributes of a figure of this shape, in canonical form.
func (s Shape) Attrs() doc.Attrs {
	return Schema().Canonical(Figure, doc.MakeAttrs(
		doc.Field{Num: FieldMedia, Value: doc.Text(s.Media)},
		doc.Field{Num: FieldWidth, Value: doc.Int(int64(min(max(s.Width, 0), 1<<16)))},
		doc.Field{Num: FieldHeight, Value: doc.Int(int64(min(max(s.Height, 0), 1<<16)))},
		doc.Field{Num: FieldSpan, Value: doc.Int(int64(min(max(s.Span, 0), SpanFull)))},
		doc.Field{Num: FieldAlign, Value: doc.Int(int64(min(max(s.Align, AlignStart), AlignEnd)))},
	))
}

// NewFigure is a figure showing a picture centered at its own size, over
// caption.
func NewFigure(s *doc.Schema, p Picture, caption string) *doc.Node {
	var text []doc.Item
	if caption != "" {
		text = append(text, doc.TextItem(caption, nil))
	}
	return vocab.NewBlock(s, Figure, Shape{Picture: p, Align: AlignCenter}.Attrs(), text...)
}

// SetShape gives a figure another shape, keeping its caption: resizing,
// aligning and replacing its picture are all this one edit.
func SetShape(id doc.ID, shape Shape) vocab.Edit {
	return vocab.SetAttrs(id, shape.Attrs())
}
