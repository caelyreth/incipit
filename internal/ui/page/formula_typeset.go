// formula_typeset sets a page's TeX as math with untie, in the math font the
// theme bundles, and draws what it sets with MyGo.
package page

import (
	"cmp"
	"context"
	"math"
	"slices"
	"strings"
	"sync"

	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/ui/components"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/render"
	untiesource "github.com/caelyreth/untie/source"
	"github.com/egoist/mygo/ui"
)

// kept bounds how many formulas stay set between layouts: more than a long
// page shows, at each of the sizes it sets them at.
const (
	formulaKept      = 1024
	formulaGlyphKept = 2048
)

var (
	formulaPrepare = sync.OnceValue(func() *untie.Renderer {
		fonts, err := untie.LoadMathFont(theme.MathFont(), untie.WithFontID("NewCMMath-Book"))
		if err != nil {
			return nil
		}
		renderer, err := untie.NewRenderer(fonts, untie.RendererOptions{CacheEntries: formulaKept})
		if err != nil {
			return nil
		}
		return renderer
	})
	formulaCache = struct {
		sync.Mutex
		boxes map[formulaKey]formulaSet
	}{boxes: make(map[formulaKey]formulaSet)}
	formulaDrawCache = struct {
		sync.Mutex
		byBox map[*untie.Box]*formulaDrawing
	}{byBox: make(map[*untie.Box]*formulaDrawing)}
	formulaGlyphCache = struct {
		sync.Mutex
		entries map[formulaGlyphKey][]cachedFormulaGlyph
		count   int
	}{entries: make(map[formulaGlyphKey][]cachedFormulaGlyph)}
)

type formulaDrawing struct {
	bounds ui.Rect
	ops    []formulaOp
}

type formulaOp struct {
	baked *ui.BakedPath
	rect  ui.Rect
	at    ui.Point
	paint untie.Paint
	rule  bool
}

type formulaGlyphKey struct {
	emBits   uint32
	hash     uint64
	segCount int
}

type cachedFormulaGlyph struct {
	segments []untie.Segment
	baked    *ui.BakedPath
}

// set is what a source set, and whether it set all of it: nothing it could
// not read, nothing missing, the problems it has else.
type formulaSet struct {
	box      *untie.Box
	sound    bool
	problems []Problem
}

// Problem is what keeps part of a source from being set: From and To are
// where, in its bytes, an empty range a place, as its end, where something
// is missing.
type Problem struct {
	From, To int
	Message  string
}

type formulaKey struct {
	source      string
	dialect     profile.MathDialect
	display     bool
	size, width float32
}

func untieDialect(d profile.MathDialect) untie.Dialect {
	if d == profile.DialectTypst {
		return untie.Typst
	}
	return untie.TeX
}

// setFormula is source typeset at size in dialect: a display formula broken into
// lines within width, an inline one as it runs in a line of text. A
// formula half typed still sets what it can, its errors marked. It is nil
// only when the math font cannot be read, when a page shows the source.
func setFormula(source string, dialect profile.MathDialect, display bool, size, width float32) *untie.Box {
	return setOf(source, dialect, display, size, width).box
}

// formulaSound reports whether source sets cleanly in dialect: no error is set
// in it, as a half typed formula's is.
func formulaSound(source string, dialect profile.MathDialect, display bool) bool {
	return setOf(source, dialect, display, soundSize, 0).sound
}

// Problems are what keep TeX source from being set as written, in order.
func Problems(source string, display bool) []Problem {
	return ProblemsFor(source, profile.DialectTeX, display)
}

// ProblemsFor are what keep source from being set in dialect as written, in order.
func ProblemsFor(source string, dialect profile.MathDialect, display bool) []Problem {
	return slices.Clone(setOf(source, dialect, display, soundSize, 0).problems)
}

// soundSize is the size Sound sets a source at: what it reads, not its
// size, decides.
const soundSize = 16

func setOf(source string, dialect profile.MathDialect, display bool, size, width float32) formulaSet {
	if !display {
		width = 0
	}
	request := formulaKey{source: source, dialect: dialect, display: display, size: size, width: width}
	formulaCache.Lock()
	found, ok := formulaCache.boxes[request]
	formulaCache.Unlock()
	if ok {
		return found
	}
	renderer := formulaPrepare()
	if renderer == nil || size <= 0 {
		return formulaSet{}
	}
	mode := untie.Inline
	style := untie.Style{Size: size}
	if display {
		mode, style.MaxWidth, style.Align = untie.Display, width, untie.AlignCenter
	}
	result, err := renderer.Render(context.Background(), source, untie.RenderRequest{Dialect: untieDialect(dialect), Mode: mode, Style: style})
	if err != nil || result == nil || result.Box == nil {
		return formulaSet{}
	}
	made := formulaSet{box: result.Box}
	// A layout repeats what the parse found: each problem is told once, as
	// the parse tells it, a command it does not know rather than the glyph
	// it then has none for.
	glyph := make([]bool, 0, 2)
	for _, diagnostic := range append(result.Diagnostics, result.Box.Diagnostics...) {
		if diagnostic.Severity != untiesource.SeverityError {
			continue
		}
		problem := Problem{From: diagnostic.Range.Start, To: diagnostic.Range.End, Message: diagnostic.Message}
		missing := diagnostic.Code == untiesource.CodeMissingGlyph
		at := slices.IndexFunc(made.problems, func(known Problem) bool {
			return known.From == problem.From && known.To == problem.To
		})
		switch {
		case at < 0:
			made.problems, glyph = append(made.problems, problem), append(glyph, missing)
		case glyph[at] && !missing:
			made.problems[at], glyph[at] = problem, false
		}
	}
	slices.SortStableFunc(made.problems, func(a, b Problem) int { return cmp.Compare(a.From, b.From) })
	made.sound = strings.TrimSpace(source) != "" && len(made.problems) == 0
	formulaCache.Lock()
	if len(formulaCache.boxes) >= formulaKept {
		clear(formulaCache.boxes)
	}
	formulaCache.boxes[request] = made
	formulaCache.Unlock()
	return made
}

// Inks are the colors a formula is drawn in: its own, as the text around
// it, what it could not read, and what stands in for what is missing.
type formulaInks struct {
	Text, Error, Placeholder ui.Color
}

// Draw draws box with its baseline's start at origin. Colors the formula
// names itself are kept.
func drawFormula(p *ui.Painter, box *untie.Box, origin ui.Point, inks formulaInks) {
	if p == nil || box == nil {
		return
	}
	drawing := cachedFormulaDrawing(box)
	if drawing == nil || len(drawing.ops) == 0 {
		return
	}
	components.WithTextOrigin(p, origin, func(origin ui.Point) {
		// The viewport is the box's ink, a pad around it so its edges keep
		// their smoothing: set from its start and centered across its height,
		// it puts the baseline's start at origin.
		const pad = 1
		viewport := ui.Rect{
			X: origin.X + drawing.bounds.X - pad, Y: origin.Y + drawing.bounds.Y - pad,
			W: drawing.bounds.W + 2*pad, H: drawing.bounds.H + 2*pad,
		}
		p.Clip(viewport, 0, func() {
			for i := range drawing.ops {
				op := &drawing.ops[i]
				color := resolveFormulaColor(inks, op.paint)
				if op.rule {
					p.Fill(ui.Rect{
						X: origin.X + op.rect.X,
						Y: origin.Y + op.rect.Y,
						W: op.rect.W,
						H: op.rect.H,
					}, color, 0)
					continue
				}
				p.FillBakedPath(op.baked, origin.X+op.at.X, origin.Y+op.at.Y, color)
			}
		})
	})
}

func resolveFormulaColor(inks formulaInks, paint untie.Paint) ui.Color {
	switch {
	case paint.HasExplicit:
		return ui.RGBA(
			paint.Explicit.R,
			paint.Explicit.G,
			paint.Explicit.B,
			float32(paint.Explicit.A)/255,
		)
	case paint.Role == untie.RoleError:
		return inks.Error
	case paint.Role == untie.RolePlaceholder:
		return inks.Placeholder
	}
	return inks.Text
}

func cachedFormulaDrawing(box *untie.Box) *formulaDrawing {
	formulaDrawCache.Lock()
	if d, ok := formulaDrawCache.byBox[box]; ok {
		formulaDrawCache.Unlock()
		return d
	}
	formulaDrawCache.Unlock()

	bounds := box.Bounds()
	d := &formulaDrawing{
		bounds: ui.Rect{X: bounds.X, Y: bounds.Y, W: bounds.Width, H: bounds.Height},
		ops:    make([]formulaOp, 0, len(box.Items)),
	}
	_ = render.Walk(box, func(cmd render.Command) error {
		switch cmd.Kind {
		case render.CommandRule:
			d.ops = append(d.ops, formulaOp{
				rule:  true,
				rect:  ui.Rect{X: cmd.Rect.X, Y: cmd.Rect.Y, W: cmd.Rect.Width, H: cmd.Rect.Height},
				paint: cmd.Paint,
			})
		case render.CommandPath:
			baked := bakeFormulaGlyph(cmd.Path.Segments, cmd.Em)
			if baked == nil {
				return nil
			}
			d.ops = append(d.ops, formulaOp{
				baked: baked,
				at:    ui.Point{X: cmd.At.X, Y: cmd.At.Y},
				paint: cmd.Paint,
			})
		}
		return nil
	})

	formulaDrawCache.Lock()
	if existing, ok := formulaDrawCache.byBox[box]; ok {
		formulaDrawCache.Unlock()
		return existing
	}
	if len(formulaDrawCache.byBox) >= formulaKept {
		clear(formulaDrawCache.byBox)
	}
	formulaDrawCache.byBox[box] = d
	formulaDrawCache.Unlock()
	return d
}

func bakeFormulaGlyph(segs []untie.Segment, em float32) *ui.BakedPath {
	if len(segs) == 0 || em <= 0 {
		return nil
	}
	key := formulaGlyphKey{
		emBits:   math.Float32bits(em),
		hash:     hashFormulaSegments(segs),
		segCount: len(segs),
	}
	formulaGlyphCache.Lock()
	for _, entry := range formulaGlyphCache.entries[key] {
		if formulaSegmentsEqual(entry.segments, segs) {
			formulaGlyphCache.Unlock()
			return entry.baked
		}
	}
	formulaGlyphCache.Unlock()

	var path ui.Path
	for _, seg := range segs {
		switch seg.Op {
		case untie.SegMoveTo:
			path.MoveTo(seg.Points[0].X*em, seg.Points[0].Y*em)
		case untie.SegLineTo:
			path.LineTo(seg.Points[0].X*em, seg.Points[0].Y*em)
		case untie.SegQuadTo:
			path.QuadTo(seg.Points[0].X*em, seg.Points[0].Y*em, seg.Points[1].X*em, seg.Points[1].Y*em)
		case untie.SegCubeTo:
			path.CubeTo(
				seg.Points[0].X*em, seg.Points[0].Y*em,
				seg.Points[1].X*em, seg.Points[1].Y*em,
				seg.Points[2].X*em, seg.Points[2].Y*em,
			)
		case untie.SegClose:
			path.Close()
		}
	}
	baked := ui.BakePath(&path)
	if baked == nil {
		return nil
	}
	cloned := slices.Clone(segs)

	formulaGlyphCache.Lock()
	for _, entry := range formulaGlyphCache.entries[key] {
		if formulaSegmentsEqual(entry.segments, segs) {
			formulaGlyphCache.Unlock()
			return entry.baked
		}
	}
	if formulaGlyphCache.count >= formulaGlyphKept {
		clear(formulaGlyphCache.entries)
		formulaGlyphCache.count = 0
	}
	formulaGlyphCache.entries[key] = append(formulaGlyphCache.entries[key], cachedFormulaGlyph{
		segments: cloned,
		baked:    baked,
	})
	formulaGlyphCache.count++
	formulaGlyphCache.Unlock()
	return baked
}

func formulaSegmentPoints(seg untie.Segment) int {
	switch seg.Op {
	case untie.SegMoveTo, untie.SegLineTo:
		return 1
	case untie.SegQuadTo:
		return 2
	case untie.SegCubeTo:
		return 3
	default:
		return 0
	}
}

func hashFormulaSegments(segs []untie.Segment) uint64 {
	const (
		offset64 = uint64(14695981039346656037)
		prime64  = uint64(1099511628211)
	)
	h := offset64
	mix := func(v uint64) {
		h ^= v
		h *= prime64
	}
	for _, seg := range segs {
		mix(uint64(seg.Op))
		n := formulaSegmentPoints(seg)
		for i := range n {
			mix(uint64(math.Float32bits(seg.Points[i].X)))
			mix(uint64(math.Float32bits(seg.Points[i].Y)))
		}
	}
	return h
}

func formulaSegmentsEqual(a, b []untie.Segment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Op != b[i].Op {
			return false
		}
		n := formulaSegmentPoints(a[i])
		for j := range n {
			if a[i].Points[j] != b[i].Points[j] {
				return false
			}
		}
	}
	return true
}

// Ink is how far across box what it draws reaches, from its baseline's
// start: within its width for a formula broken into lines, centered there.
func formulaInk(box *untie.Box) (from, to float32) {
	if box == nil {
		return 0, 0
	}
	from, to = box.Width, 0
	for _, item := range box.Items {
		if item.Bound.Width <= 0 {
			continue
		}
		from, to = min(from, item.Bound.X), max(to, item.Bound.X+item.Bound.Width)
	}
	if from > to {
		return 0, box.Width
	}
	return from, to
}

// Extent is how far above and below its baseline box draws: its ascent and
// descent, or further where its ink reaches past them, as a limit set
// below an integral does.
func formulaExtent(box *untie.Box) (ascent, descent float32) {
	if box == nil {
		return 0, 0
	}
	bounds := box.Bounds()
	return max(box.Ascent, -bounds.Y), max(box.Descent, bounds.Y+bounds.Height)
}
