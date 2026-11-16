package shell

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type fieldState struct {
	key        fieldCacheKey
	layout     stage.FieldLayout
	cards      map[string]fieldCardData
	selectedID string
	dragging   bool
	scrolledAt time.Time
	panImpulse stage.Point
	damped     stage.Point
}

type fieldCacheKey struct {
	hasLib       bool
	libRev       uint64
	openKey      string
	activePage   string
	scopeGroup   string
	fidelityStep int
	query        string
	posture      stage.Posture
	hasDesk      bool
	vpW          float32
	vpH          float32
	head         float32
	uiFontSize   float32
}

type fieldCardData struct {
	kind          stage.CardKind
	group         GroupItem
	sheet         SheetItem
	matchLine     string
	previewTitles []string
	totalWords    int
	onDesk        bool
	activeOnDesk  bool
	trashed       bool
}

const (
	fieldGroupCardPrefix = "group:"
	fieldSheetCardPrefix = "sheet:"
	fieldNewCardID       = "new:sheet"
)

// FieldPan returns the current field pan offset.
func (s *State) FieldPan() stage.Point {
	if s == nil {
		return stage.Point{}
	}
	return s.pill.pan
}

// PanField pans the field by (dx, dy) DIPs and clamps/resists past bounds.
func (s *State) PanField(dx, dy float32) {
	if s == nil {
		return
	}
	scale := s.Settings.UIFontSize
	if scale <= 0 {
		scale = theme.DefaultUIFontSize
	}
	k := scale / theme.DefaultUIFontSize
	s.panField(s.field.layout, dx, dy, k)
}

// FieldLayout returns the most recently computed field card layout.
func (s *State) FieldLayout() stage.FieldLayout {
	if s == nil {
		return stage.FieldLayout{}
	}
	return s.field.layout
}

// PillScope returns the active group scope ID on the pill, or "" for overview.
func (s *State) PillScope() string {
	if s == nil {
		return ""
	}
	return s.pill.scopeGroup
}

// SetPillScope sets the active group scope on the pill and recenters the pan.
func (s *State) SetPillScope(group string) {
	if s == nil {
		return
	}
	s.pill.scopeGroup = group
	s.pill.fidelityStep = 0
	s.pill.pan = stage.Point{}
	s.field.selectedID = ""
}

// FidelityStep returns the manual fidelity step offset on the field pill.
func (s *State) FidelityStep() int {
	if s == nil {
		return 0
	}
	return s.pill.fidelityStep
}

// SetFidelityStep sets the manual fidelity step (0 = overview/group, 1 = sheets).
func (s *State) SetFidelityStep(step int) {
	if s == nil {
		return
	}
	step = max(0, min(step, 1))
	if s.pill.fidelityStep == step {
		return
	}
	s.pill.fidelityStep = step
	s.pill.pan = stage.Point{}
	s.field.selectedID = ""
}

// StepFidelity steps field fidelity down (delta > 0) or up (delta < 0).
func (s *State) StepFidelity(delta int) {
	if s == nil {
		return
	}
	s.SetFidelityStep(s.pill.fidelityStep + delta)
}

// PillFidelity returns the currently resolved fidelity level for the field.
func (s *State) PillFidelity() stage.Fidelity {
	if s == nil {
		return stage.FidelityOverview
	}
	return stage.ResolveFidelity(s.pill.scopeGroup, strings.TrimSpace(s.pill.query), s.pill.fidelityStep)
}

func (s *State) fieldCardsView(c *ui.Context, host ui.Element, semantic theme.Theme, scene stage.Scene, width, height, head float32, animate bool) {
	scale := s.Settings.UIFontSize
	k := scale / theme.DefaultUIFontSize
	cell := 156 * k
	gap := 14 * k
	layout := s.ensureFieldLayout(c, scene, width, height, head, cell, gap, k)

	// Clamp target pan within the packed field bounds when not dragging.
	if !s.field.dragging {
		s.pill.pan = stage.ClampPan(layout, s.pill.pan, 64*k)
	}
	now := c.Now()
	directScroll := !s.field.scrolledAt.IsZero() && now.Sub(s.field.scrolledAt) < 90*time.Millisecond
	if directScroll {
		// Re-check once scroll quiets so overscroll settles cleanly.
		c.After(90*time.Millisecond - now.Sub(s.field.scrolledAt))
	}
	panX, panY := s.pill.pan.X, s.pill.pan.Y
	if animate {
		if s.field.dragging || directScroll {
			// Keep 1:1 pointer tracking while dragging/scrolling while seeding
			// the animation state so fling release continues seamlessly.
			panX = host.AnimateTo("field-pan-x", s.pill.pan.X, ui.Animation{Duration: 1})
			panY = host.AnimateTo("field-pan-y", s.pill.pan.Y, ui.Animation{Duration: 1})
		} else {
			panX = components.Follow(host, "field-pan-x", s.pill.pan.X, components.MotionPanel)
			panY = components.Follow(host, "field-pan-y", s.pill.pan.Y, components.MotionPanel)
		}
		s.field.damped = stage.Point{
			X: components.Follow(host, "pill-damped-x", s.field.panImpulse.X, components.MotionQuick),
			Y: components.Follow(host, "pill-damped-y", s.field.panImpulse.Y, components.MotionQuick),
		}
		// Decay the impulse back to zero after reading it so the pill springs back.
		s.field.panImpulse = stage.Point{}
	} else {
		s.field.damped = stage.Point{}
		s.field.panImpulse = stage.Point{}
	}
	pan := stage.Point{X: panX, Y: panY}

	// Background drag-to-pan surface behind cards.
	panPad := ui.Box(c).Key("field-pan").Absolute().Fill()
	dx, dy, dragging := panPad.Dragged()
	s.field.dragging = dragging
	if dragging && (dx != 0 || dy != 0) {
		s.panField(layout, dx, dy, k)
	}
	if vx, vy, ended := panPad.DragEnded(); ended {
		s.field.dragging = false
		carry := stage.Point{
			X: s.pill.pan.X + vx*flingCarry,
			Y: s.pill.pan.Y + vy*flingCarry,
		}
		s.pill.pan = stage.ClampPan(layout, carry, 64*k)
	}

	// Cull to the visible viewport with a generous margin so cards never pop
	// while translating on the GPU pan layer.
	viewRect := scene.Field.Rect.Inset(-cell*1.5, -cell*1.5)
	visible := stage.VisibleCards(layout, viewRect, pan)
	// Draw back-to-front by Depth emphasis, active desk status, and selection
	// so emphasized cards step visually in front of receded neighbors.
	slices.SortStableFunc(visible, func(a, b stage.PlacedCard) int {
		if c := cmp.Compare(a.Emphasis.Depth, b.Emphasis.Depth); c != 0 {
			return c
		}
		ad, bd := s.field.cards[a.Item.ID], s.field.cards[b.Item.ID]
		ap := boolRank(ad.activeOnDesk) + 2*boolRank(a.Item.ID == s.field.selectedID)
		bp := boolRank(bd.activeOnDesk) + 2*boolRank(b.Item.ID == s.field.selectedID)
		return cmp.Compare(ap, bp)
	})

	ui.Box(c).Key("field-cards").Absolute().Fill().PassThrough().
		Translate(pan.X, pan.Y).Children(func() {
		for _, card := range visible {
			s.fieldCardView(c, semantic, card, k, now, animate)
		}
	})
}

func (s *State) panField(layout stage.FieldLayout, dx, dy, k float32) {
	next := stage.Point{X: s.pill.pan.X + dx, Y: s.pill.pan.Y + dy}
	clamped := stage.ClampPan(layout, next, 64*k)
	// Elastic resistance beyond bounds while panning.
	if next.X != clamped.X {
		next.X = clamped.X + (next.X-clamped.X)*0.3
	}
	if next.Y != clamped.Y {
		next.Y = clamped.Y + (next.Y-clamped.Y)*0.3
	}
	s.pill.pan = next
	s.field.panImpulse = stage.Point{
		X: max(-8*k, min(dx*0.35, 8*k)),
		Y: max(-8*k, min(dy*0.35, 8*k)),
	}
}

func (s *State) ensureFieldLayout(c *ui.Context, scene stage.Scene, width, height, head, cell, gap, k float32) stage.FieldLayout {
	// Keep the last packed layout frozen while returning to Writing so closing
	// transitions never re-pack cards onto the title line.
	packPosture := s.posture
	if packPosture == stage.PostureWriting && s.field.cards != nil {
		return s.field.layout
	}
	if packPosture == stage.PostureWriting {
		packPosture = stage.PostureGlance
	}

	openKey := openPagesKey(s.Open)
	hasDesk := len(s.Open) > 0
	animatingPosture := math.Abs(float64(s.postureLevel-postureTarget(s.posture))) > 0.002

	var key fieldCacheKey
	key.hasLib = s.Library != nil
	key.openKey = openKey
	key.activePage = s.ActivePage
	key.scopeGroup = s.pill.scopeGroup
	key.fidelityStep = s.pill.fidelityStep
	key.query = s.pill.query
	key.posture = packPosture
	key.hasDesk = hasDesk
	key.vpW = width
	key.vpH = height
	key.head = head
	key.uiFontSize = s.Settings.UIFontSize

	// Skip re-scanning the library on active pan or posture-flight frames
	// unless structural inputs already missed the cache.
	baseMatch := s.field.cards != nil &&
		key.hasLib == s.field.key.hasLib &&
		key.openKey == s.field.key.openKey &&
		key.activePage == s.field.key.activePage &&
		key.scopeGroup == s.field.key.scopeGroup &&
		key.fidelityStep == s.field.key.fidelityStep &&
		key.query == s.field.key.query &&
		key.posture == s.field.key.posture &&
		key.hasDesk == s.field.key.hasDesk &&
		key.vpW == s.field.key.vpW &&
		key.vpH == s.field.key.vpH &&
		key.head == s.field.key.head &&
		key.uiFontSize == s.field.key.uiFontSize
	if baseMatch && (s.field.dragging || animatingPosture) {
		return s.field.layout
	}

	key.libRev = s.libraryFingerprint()
	if key == s.field.key && s.field.cards != nil {
		return s.field.layout
	}

	targetScene := stage.Stage(stage.Rect{W: width, H: height}, packPosture, hasDesk, head)
	items, cards := s.buildFieldItems(c.Now(), targetScene.FieldRoom)
	roomW := targetScene.FieldRoom.W
	if roomW <= 0 {
		roomW = width
	}
	pillW := min(392*k, max(240*k, roomW-48*k))
	pillH := 124 * k
	layout := stage.PackField(stage.FieldSpec{
		Room:       targetScene.FieldRoom,
		PillAnchor: targetScene.PillAnchor,
		PillSize:   stage.Size{W: pillW, H: pillH},
		Cell:       cell,
		Gap:        gap,
	}, items)

	s.field.key = key
	s.field.layout = layout
	s.field.cards = cards
	return layout
}

func (s *State) libraryFingerprint() uint64 {
	if s.Library == nil {
		return 0
	}
	var h uint64 = 1469598103934665603
	mix := func(v string) {
		for i := 0; i < len(v); i++ {
			h ^= uint64(v[i])
			h *= 1099511628211
		}
		h ^= 0xff
		h *= 1099511628211
	}
	for _, g := range s.Library.Groups(s.pill.scopeGroup) {
		mix(g.ID)
		mix(g.Name)
		h ^= uint64(g.Count)*31 + uint64(g.Color)*131
	}
	place := Place{Kind: PlaceAll}
	if s.pill.scopeGroup != "" {
		place = Place{Kind: PlaceGroup, Group: s.pill.scopeGroup}
	}
	for _, sh := range s.Library.Sheets(place) {
		mix(sh.ID)
		mix(sh.Title)
		h ^= uint64(sh.Words) * 17
		h ^= uint64(sh.Updated.UnixNano())
	}
	return h
}

func (s *State) buildFieldItems(now time.Time, _ stage.Rect) ([]stage.FieldItem, map[string]fieldCardData) {
	cards := map[string]fieldCardData{}
	if s.Library == nil {
		return nil, cards
	}
	openSet := make(map[string]bool, len(s.Open))
	for _, p := range s.Open {
		openSet[p.ID] = true
	}

	q := strings.TrimSpace(s.pill.query)
	hasQuery := q != ""
	parsed := query.Parse(q)
	trashMode := queryHasFilter(parsed, "trash")
	fidelity := stage.ResolveFidelity(s.pill.scopeGroup, q, s.pill.fidelityStep)

	var scopedSheets map[string]bool
	if s.pill.scopeGroup != "" {
		scopedSheets = s.scopedSheetSet(s.pill.scopeGroup)
	}

	matches := map[string]float32{}
	if hasQuery {
		found := s.Library.Search(s.pill.query)
		n := len(found.Sheets)
		rank := 0
		for _, sh := range found.Sheets {
			if scopedSheets != nil && !scopedSheets[sh.ID] {
				continue
			}
			score := float32(1)
			if n > 1 {
				score = 1 - 0.45*float32(rank)/float32(n-1)
			}
			rank++
			matches[sh.ID] = score
			cards[fieldSheetCardPrefix+sh.ID] = fieldCardData{
				kind:         stage.CardSheet,
				sheet:        sh.SheetItem,
				matchLine:    sh.Line,
				onDesk:       openSet[sh.ID],
				activeOnDesk: sh.ID == s.ActivePage,
				trashed:      trashMode,
			}
		}
	}

	var items []stage.FieldItem

	addGroup := func(g GroupItem) {
		sheets := s.Library.Sheets(Place{Kind: PlaceGroup, Group: g.ID})
		var totalWords int
		var latest time.Time
		var onDesk, active bool
		previews := make([]string, 0, 3)
		for _, sh := range sheets {
			totalWords += sh.Words
			if sh.Updated.After(latest) {
				latest = sh.Updated
			}
			if openSet[sh.ID] {
				onDesk = true
			}
			if sh.ID == s.ActivePage {
				active = true
			}
			if len(previews) < 3 && sh.Title != "" {
				previews = append(previews, sh.Title)
			}
		}
		ago := time.Duration(0)
		if !latest.IsZero() && now.After(latest) {
			ago = now.Sub(latest)
		} else if latest.IsZero() {
			ago = 30 * 24 * time.Hour
		}
		sig := stage.Signal{
			ActiveOnDesk:  active,
			OnDesk:        onDesk,
			InActiveScope: g.ID == s.pill.scopeGroup,
			UpdatedAgo:    ago,
			Words:         totalWords,
		}
		id := fieldGroupCardPrefix + g.ID
		cards[id] = fieldCardData{
			kind:          stage.CardGroup,
			group:         g,
			previewTitles: previews,
			totalWords:    totalWords,
			onDesk:        onDesk,
			activeOnDesk:  active,
		}
		items = append(items, stage.FieldItem{
			ID:     id,
			Kind:   stage.CardGroup,
			Size:   stage.CardSizeClass(stage.CardGroup, fidelity, sig, g.Count),
			Signal: sig,
		})
	}

	addSheet := func(sh SheetItem, inScope, trashed bool) {
		ago := time.Duration(0)
		if !sh.Updated.IsZero() && now.After(sh.Updated) {
			ago = now.Sub(sh.Updated)
		} else if sh.Updated.IsZero() {
			ago = 30 * 24 * time.Hour
		}
		sig := stage.Signal{
			ActiveOnDesk:  sh.ID == s.ActivePage,
			OnDesk:        openSet[sh.ID],
			InActiveScope: inScope,
			UpdatedAgo:    ago,
			Words:         sh.Words,
		}
		id := fieldSheetCardPrefix + sh.ID
		if _, exists := cards[id]; !exists {
			cards[id] = fieldCardData{
				kind:         stage.CardSheet,
				sheet:        sh,
				onDesk:       openSet[sh.ID],
				activeOnDesk: sh.ID == s.ActivePage,
				trashed:      trashed,
			}
		}
		items = append(items, stage.FieldItem{
			ID:       id,
			Kind:     stage.CardSheet,
			Size:     stage.CardSizeClass(stage.CardSheet, fidelity, sig, 0),
			Signal:   sig,
			Match:    matches[sh.ID],
			HasQuery: hasQuery,
		})
	}

	if trashMode {
		for _, sh := range s.Library.Sheets(Place{Kind: PlaceTrash}) {
			addSheet(sh, false, true)
		}
		return items, cards
	}

	switch fidelity {
	case stage.FidelityOverview:
		for _, g := range s.Library.Groups("") {
			addGroup(g)
		}
		// Include high-nearness overview sheets (open on desk + top recent).
		all := s.Library.Sheets(Place{Kind: PlaceAll})
		slices.SortStableFunc(all, func(a, b SheetItem) int {
			oa, ob := openSet[a.ID], openSet[b.ID]
			if oa != ob {
				if oa {
					return -1
				}
				return 1
			}
			return b.Updated.Compare(a.Updated)
		})
		limit := min(6, len(all))
		for _, sh := range all[:limit] {
			addSheet(sh, false, false)
		}
	case stage.FidelityGroup:
		for _, sub := range s.Library.Groups(s.pill.scopeGroup) {
			addGroup(sub)
		}
		for _, sh := range s.Library.Sheets(Place{Kind: PlaceGroup, Group: s.pill.scopeGroup}) {
			addSheet(sh, true, false)
		}
	default: // FidelitySheets
		place := Place{Kind: PlaceAll}
		inScope := false
		if s.pill.scopeGroup != "" {
			place = Place{Kind: PlaceGroup, Group: s.pill.scopeGroup}
			inScope = true
		}
		seen := map[string]bool{}
		for _, sh := range s.Library.Sheets(place) {
			seen[sh.ID] = true
			addSheet(sh, inScope, false)
		}
		if hasQuery && s.pill.scopeGroup == "" {
			for id, card := range cards {
				if card.kind == stage.CardSheet && !seen[card.sheet.ID] && strings.HasPrefix(id, fieldSheetCardPrefix) {
					addSheet(card.sheet, false, false)
				}
			}
		}
	}

	if !hasQuery {
		cards[fieldNewCardID] = fieldCardData{kind: stage.CardNew}
		items = append(items, stage.FieldItem{
			ID:     fieldNewCardID,
			Kind:   stage.CardNew,
			Size:   stage.Size1x1,
			Signal: stage.Signal{UpdatedAgo: 14 * 24 * time.Hour},
		})
	}
	return items, cards
}

func (s *State) fieldCardView(
	c *ui.Context,
	semantic theme.Theme,
	card stage.PlacedCard,
	k float32,
	now time.Time,
	animate bool,
) {
	data, ok := s.field.cards[card.Item.ID]
	if !ok {
		return
	}
	selected := s.field.selectedID == card.Item.ID
	targetScale := card.Emphasis.Scale
	if targetScale <= 0 {
		targetScale = 1
	}

	var label string
	switch data.kind {
	case stage.CardGroup:
		label = data.group.Name
	case stage.CardSheet:
		label = data.sheet.Title
		if label == "" {
			label = "Untitled"
		}
	case stage.CardNew:
		label = "New sheet"
	}

	btn := ui.ButtonBase(c.Key("field-card-" + card.Item.ID)).Absolute().
		Left(card.Rect.X).Top(card.Rect.Y).
		Radius(12 * k).Padding(12 * k).Clip().
		FocusRing(false).KeepFocus().Label(label).Cache()
	if animate {
		btn.Transition(ui.ElementTransition{
			Duration: motionEnter,
			Enter:    &ui.Motion{Scale: 0.94, Opacity: 0},
		})
	}

	curW, curH := card.Rect.W, card.Rect.H
	scale := targetScale
	opacity := card.Emphasis.Opacity
	shiftX, shiftY := float32(0), float32(0)
	hover := float32(0)
	if btn.Hovered() {
		hover = 1
	}
	if animate {
		animX := components.Follow(btn, "card-x", card.Rect.X, components.MotionPanel)
		animY := components.Follow(btn, "card-y", card.Rect.Y, components.MotionPanel)
		curW = components.Follow(btn, "card-w", card.Rect.W, components.MotionPanel)
		curH = components.Follow(btn, "card-h", card.Rect.H, components.MotionPanel)
		shiftX = animX - card.Rect.X
		shiftY = animY - card.Rect.Y
		scale = components.Follow(btn, "card-scale", targetScale, components.MotionQuick)
		opacity = components.Follow(btn, "card-opacity", card.Emphasis.Opacity, components.MotionQuick)
		hover = components.HoverAmount(btn, btn.Hovered(), true)
	}
	btn.Size(curW, curH).TransformOrigin(0.5, 0.5).
		Translate(shiftX, shiftY).
		Scale(scale, scale).
		Opacity(max(0, min(1, opacity)))

	rest := semantic.Surface.Page
	border := semantic.Border.Variant
	var borderW float32 = 1
	if data.kind == stage.CardNew {
		rest = semantic.Surface.Inset
	}
	if b := card.Emphasis.Brightness; b < 1 {
		rest = components.Blend(rest, semantic.Surface.Background, max(0, min(1, 1-b)))
	} else if b > 1 && !selected && !data.activeOnDesk {
		border = components.Blend(border, semantic.Accent.Foreground, max(0, min(0.65, (b-1)*4)))
	}
	bg := components.Blend(rest, semantic.Element.Hover, hover)
	if selected {
		bg = semantic.Surface.Chosen
		border = semantic.Border.Focused
		borderW = 1.5
	} else if data.activeOnDesk {
		border = semantic.Accent.Foreground
	}
	btn.Background(bg).Border(borderW, border)

	if btn.Clicked() {
		alongside := btn.ClickModifiers()&(ui.Cmd|ui.Alt) != 0
		s.activateFieldCard(card.Item.ID, alongside)
	}

	btn.Children(func() {
		switch data.kind {
		case stage.CardGroup:
			s.groupBoardCard(c, semantic, data, k)
		case stage.CardSheet:
			s.sheetFieldCard(c, semantic, data, card.GridW > 1, card.GridH > 1, k, now)
		case stage.CardNew:
			s.newSheetFieldCard(c, semantic, k)
		}
	})
}

func (s *State) groupBoardCard(c *ui.Context, semantic theme.Theme, data fieldCardData, k float32) {
	ui.Column(c).Fill().Gap(6 * k).Children(func() {
		ui.Row(c).WidthPercent(100).AlignItems(ui.Center).Gap(6 * k).Children(func() {
			ink := semantic.Icon.Muted
			if tint := groupTint(semantic, data.group); tint != nil {
				ink = *tint
			}
			ui.Icon(c, groupIcon(data.group).SVG()).FontSize(14 * k).TextColor(ink).Shrink(0)
			components.Label(c, data.group.Name, theme.TextSmall, semantic.Text.Heading).
				Grow(1).MinWidth(0).SingleLine()
			components.Label(c, fmt.Sprint(data.group.Count), theme.TextXSmall, semantic.Text.Muted).
				SingleLine().Shrink(0).FontFeatures("tnum")
		})
		if len(data.previewTitles) > 0 {
			ui.Column(c).Grow(1).MinHeight(0).Gap(3 * k).Children(func() {
				for _, title := range data.previewTitles {
					components.Label(c, "· "+title, theme.TextXSmall, semantic.Text.Muted).
						WidthPercent(100).SingleLine()
				}
			})
		} else {
			ui.Box(c).Grow(1)
		}
		if data.totalWords > 0 {
			components.Label(c, words(data.totalWords), theme.TextXSmall, semantic.Text.Muted).
				SingleLine().FontFeatures("tnum").Opacity(0.75)
		}
	})
}

func (s *State) sheetFieldCard(c *ui.Context, semantic theme.Theme, data fieldCardData, wide, tall bool, k float32, now time.Time) {
	title := data.sheet.Title
	if title == "" {
		title = "Untitled"
	}
	titleColor := semantic.Text.Default
	if data.activeOnDesk {
		titleColor = semantic.Text.Heading
	}
	ui.Column(c).Fill().Gap(5 * k).Children(func() {
		ui.Row(c).WidthPercent(100).AlignItems(ui.Center).Gap(6 * k).Children(func() {
			if data.onDesk {
				dotColor := semantic.Text.Muted
				if data.activeOnDesk {
					dotColor = semantic.Accent.Foreground
				}
				ui.Box(c).Size(6*k, 6*k).Radius(3 * k).Background(dotColor).Shrink(0)
			}
			components.Label(c, title, theme.TextSmall, titleColor).
				Grow(1).MinWidth(0).SingleLine()
			if !data.sheet.Updated.IsZero() {
				components.Label(c, when(data.sheet.Updated, now), theme.TextXSmall, semantic.Text.Muted).
					SingleLine().Shrink(0).FontFeatures("tnum")
			}
		})
		body := data.matchLine
		if body == "" {
			body = data.sheet.Excerpt
		}
		if body != "" {
			lines := 2
			if wide {
				lines = 3
			}
			if tall {
				lines += 2
			}
			components.Label(c, body, theme.TextXSmall, semantic.Text.Muted).
				WidthPercent(100).MaxLines(lines)
		}
		ui.Box(c).Grow(1)
		ui.Row(c).WidthPercent(100).AlignItems(ui.Center).Gap(4 * k).Children(func() {
			components.Label(c, words(data.sheet.Words), theme.TextXSmall, semantic.Text.Muted).
				SingleLine().FontFeatures("tnum").Opacity(0.75)
			ui.Box(c).Grow(1)
			limit := min(2, len(data.sheet.Tags))
			for _, tag := range data.sheet.Tags[:limit] {
				components.TagChip(c, tag, theme.TextXSmall, 4*k, 72*k, semantic.Tag.Background, semantic.Tag.Foreground)
			}
		})
	})
}

func (s *State) newSheetFieldCard(c *ui.Context, semantic theme.Theme, k float32) {
	ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Gap(6 * k).Children(func() {
		ui.Icon(c, components.IconNewSheet.SVG()).FontSize(18 * k).TextColor(semantic.Icon.Muted)
		components.Label(c, "New sheet", theme.TextXSmall, semantic.Text.Muted).SingleLine()
	})
}

func (s *State) activateFieldCard(id string, alongside bool) {
	data, ok := s.field.cards[id]
	if !ok || s.Library == nil {
		return
	}
	switch data.kind {
	case stage.CardGroup:
		s.pill.scopeGroup = data.group.ID
		s.pill.pan = stage.Point{}
		s.field.selectedID = ""
		s.pill.focus = true
	case stage.CardSheet:
		if strings.TrimSpace(s.pill.query) != "" {
			s.Library.OpenFound(data.sheet.ID, s.pill.query, alongside)
		} else {
			s.Library.OpenSheet(data.sheet.ID, alongside)
		}
		if !alongside {
			s.ReturnToDesk()
		}
	case stage.CardNew:
		group := s.pill.scopeGroup
		if group == "" {
			group = s.CurrentGroup()
		}
		s.Library.NewSheet(group, Last)
		s.ReturnToDesk()
	}
}

func (s *State) activateSelectedOrNearestCard(alongside bool) {
	if s.field.selectedID != "" {
		if _, ok := s.field.cards[s.field.selectedID]; ok {
			s.activateFieldCard(s.field.selectedID, alongside)
			return
		}
	}
	// Otherwise pick the highest-priority matched/nearest actionable card.
	for _, card := range s.field.layout.Cards {
		if card.Item.Kind == stage.CardNew {
			continue
		}
		if card.Item.HasQuery && card.Item.Match <= 0 {
			continue
		}
		s.activateFieldCard(card.Item.ID, alongside)
		return
	}
	if len(s.field.layout.Cards) > 0 {
		s.activateFieldCard(s.field.layout.Cards[0].Item.ID, alongside)
	}
}

func (s *State) moveFieldSelection(dx, dy float32) {
	cards := s.field.layout.Cards
	if len(cards) == 0 {
		return
	}
	curIdx := slices.IndexFunc(cards, func(c stage.PlacedCard) bool { return c.Item.ID == s.field.selectedID })
	if curIdx < 0 {
		s.field.selectedID = cards[0].Item.ID
		return
	}
	cur := cards[curIdx].Rect.Center()
	bestIdx := -1
	bestScore := math.MaxFloat64
	for i, cand := range cards {
		if i == curIdx {
			continue
		}
		p := cand.Rect.Center()
		vx, vy := float64(p.X-cur.X), float64(p.Y-cur.Y)
		proj := vx*float64(dx) + vy*float64(dy)
		if proj <= 1 {
			continue
		}
		perp := math.Abs(vx*float64(dy) - vy*float64(dx))
		score := proj + 2.5*perp
		if score < bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	if bestIdx >= 0 {
		s.field.selectedID = cards[bestIdx].Item.ID
	}
}

func openPagesKey(open []Page) string {
	if len(open) == 0 {
		return ""
	}
	var b strings.Builder
	for i, p := range open {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(p.ID)
	}
	return b.String()
}

func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *State) scopedSheetSet(group string) map[string]bool {
	set := map[string]bool{}
	if s.Library == nil || group == "" {
		return set
	}
	var walk func(id string)
	seen := map[string]bool{}
	walk = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		for _, sh := range s.Library.Sheets(Place{Kind: PlaceGroup, Group: id}) {
			set[sh.ID] = true
		}
		for _, sub := range s.Library.Groups(id) {
			walk(sub.ID)
		}
	}
	walk(group)
	return set
}

func queryHasFilter(q query.Query, keyName string) bool {
	for _, cond := range query.Conditions(q) {
		if cond.Key != nil && cond.Key.Name == keyName && !cond.Not {
			return true
		}
	}
	return false
}
