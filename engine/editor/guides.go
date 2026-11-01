package editor

import (
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// Guide is a producer's offer of a command at a visible landing point:
// invoking Command now would take the caret to At. A guide names no keys of
// its own; the frame shows the command's configured binding, so rebinding
// or unbinding the command moves or removes every guide for it.
type Guide struct {
	Producer surface.ProducerID
	// Local distinguishes one producer's guides. A guide that keeps its
	// identity while its destination changes glides there.
	Local   uint64
	Command kernel.CommandID
	At      model.UTF16Offset
	// Area, when set, is the region invoking Command would select: the
	// guide marks that area, with its keys, rather than the point At.
	Area      []model.UTF16Range
	ExpiresAt *time.Time
}

// GuideKeys formats a configured binding as the keycaps a guide shows. The
// application owns notation; the engine only resolves which binding.
type GuideKeys func(kernel.BindingTarget) []string

// guideProducer reports the guides one feature offers for the current
// selection. Producers gate themselves, on their own policies and on the
// selection they offer for.
type guideProducer func(*Session, time.Time) []Guide

var guideProducers = []guideProducer{
	(*Session).navigationGuides,
	(*Session).scopeGuides,
}

// Guides returns every guide offered now, before key resolution.
func (s *Session) Guides(now time.Time) []Guide {
	guides := make([]Guide, 0, len(guideProducers))
	for _, produce := range guideProducers {
		guides = append(guides, produce(s, now)...)
	}
	return guides
}

// guideDecorations resolves guides to keycap decorations. A guide whose
// command has no binding, or whose binding formats to nothing, is dropped:
// a hint must never show a shortcut that would not work.
func (s *Session) guideDecorations(now time.Time, keys GuideKeys) []surface.Decoration {
	if keys == nil {
		return nil
	}
	configuration := s.runtime.Configuration()
	decorations := make([]surface.Decoration, 0)
	for _, guide := range s.Guides(now) {
		bindings := configuration.BindingsFor(guide.Command)
		if len(bindings) == 0 {
			continue
		}
		caps := keys(bindings[0])
		if len(caps) == 0 {
			continue
		}
		primitive := surface.GuidePrimitive(guide.At, caps, surface.ColorMuted)
		if len(guide.Area) > 0 {
			primitive = surface.AreaPrimitive(guide.Area, caps, surface.ColorMuted)
		}
		decorations = append(decorations, surface.Decoration{
			ID:         surface.DecorationID{Producer: guide.Producer, Local: guide.Local},
			Primitive:  primitive,
			Opacity:    surface.FixedOpacity(255),
			Visibility: surface.VisibilityFocused,
			ExpiresAt:  guide.ExpiresAt,
		})
	}
	return decorations
}

// navigationGuides offers Tab where it would land: past the enclosing pair
// or emphasis, or back at the last edit. One identity across offer kinds
// lets the guide glide between destinations instead of blinking.
func (s *Session) navigationGuides(now time.Time) []Guide {
	if !s.presentation.VisualCollapsed() {
		return nil
	}
	offer, ok, err := s.TabOffer(now)
	if err != nil || !ok {
		return nil
	}
	target, err := s.tabDestination(offer)
	if err != nil {
		return nil
	}
	guide := Guide{
		Producer: surface.ProducerID(NavigationPolicyID),
		Command:  TabCommand().ID,
		At:       target.RangeUTF16().Start,
	}
	if offer.HasExpiry {
		expires := offer.ExpiresAt
		guide.ExpiresAt = &expires
	}
	return []Guide{guide}
}
