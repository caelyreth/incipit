package shell

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestSheetDropFrameUnfoldsLikeCreateAreas(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		scale   float32
		room    bool
		reduced bool
		animate bool
	}{
		{"sheet", 1, false, false, true},
		{"retina", 2, false, false, true},
		{"chapter", 1, true, false, true},
		{"reduced motion", 1, false, true, true},
		{"animation disabled", 1, false, false, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			state := New()
			semantic := state.Settings.Semantic()
			height := theme.DefaultControls.Scaled(state.Settings.UIFontSize).Height
			state.list.sheetDropHeight = height * 2
			if testCase.room {
				state.list.sheetDropRoom = chapterRoom(state.Settings.UIFontSize)
			}
			dropHeight := state.list.sheetDropHeight + state.list.sheetDropRoom
			shown, active := false, true
			tester := ui.NewTesterWith(func(c *ui.Context) {
				c.SetTheme(state.Settings.MyGoTheme())
				ui.Row(c).Fill().AlignItems(ui.Start).Children(func() {
					ui.Column(c).Width(200).HeightPercent(100).Children(func() {
						if state.list.sheetDropVisible {
							state.sheets().sheetDropPlaceholder(c, semantic, active, testCase.animate).Label("Drop target")
						}
					})
					ui.Column(c).Width(200).HeightPercent(100).Children(func() {
						if shown {
							state.createPair(c, semantic, "creator", testCase.animate, nil, nil).
								Transition(unfoldTransition(testCase.animate)).Label("Creator")
						}
					})
				})
			}, 400, 240, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: testCase.reduced}})
			tester.SetScale(testCase.scale)
			positions := func() (float32, float32) {
				drop, _ := tester.Find("Drop target")
				creator, _ := tester.Find("Creator")
				return drop.H / dropHeight, creator.H / height
			}
			shown, state.list.sheetDropVisible = true, true
			tester.Frame()
			instant := testCase.reduced || !testCase.animate
			if drop, creator := positions(); instant {
				if abs(drop-1) > 0.001 || abs(creator-1) > 0.001 {
					t.Fatalf("motion policy did not show both targets immediately: drop %v, creator %v", drop, creator)
				}
			} else {
				if drop != 0 || creator != 0 {
					t.Fatalf("a target snapped open: drop %v, creator %v", drop, creator)
				}
				for _, elapsed := range []time.Duration{32 * time.Millisecond, 48 * time.Millisecond} {
					tester.Advance(elapsed)
					tester.Frame()
					drop, creator := positions()
					if drop <= 0 || drop >= 1 || abs(drop-creator) > 0.001 {
						t.Fatalf("drop and creator did not unfold together: drop %v, creator %v", drop, creator)
					}
				}
			}
			tester.Advance(motionList)
			tester.Frame()
			if drop, creator := positions(); abs(drop-1) > 0.001 || abs(creator-1) > 0.001 {
				t.Fatalf("the targets did not finish unfolding: drop %v, creator %v", drop, creator)
			}
			active = false
			tester.Frame()
			if !instant {
				tester.Advance(32 * time.Millisecond)
				tester.Frame()
				before, _ := positions()
				if before <= 0 || before >= 1 {
					t.Fatalf("the target snapped closed: %v", before)
				}
				active = true
				tester.Frame()
				if after, _ := positions(); abs(after-before) > 0.001 {
					t.Fatalf("reentering during collapse jumped from %v to %v", before, after)
				}
				tester.Advance(motionList)
				tester.Frame()
				active = false
				tester.Frame()
			}
			tester.Advance(motionList + time.Millisecond)
			tester.Frame()
			if drop, _ := positions(); drop > 0.001 || state.list.sheetDropVisible {
				t.Fatalf("the target did not finish closing: height %v, visible %v", drop, state.list.sheetDropVisible)
			}
		})
	}
}
