package showcase

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

func defaultConfiguration(t *testing.T) kernel.PolicyConfiguration {
	t.Helper()
	catalog, err := editor.EditorPolicyCatalog()
	if err != nil {
		t.Fatal(err)
	}
	host, err := kernel.NewPolicyHost(catalog)
	if err != nil {
		t.Fatal(err)
	}
	return host.Configuration()
}

// playLoop advances player beat by beat through one loop and returns the
// page as it stood when the loop ended.
func playLoop(t *testing.T, player *Player, now time.Time) (string, time.Time) {
	t.Helper()
	loop := player.Loop()
	for steps := 0; steps < 10_000; steps++ {
		text := player.Document().Text()
		next := player.Advance(now)
		if player.Loop() != loop {
			return text, now
		}
		if err := player.Err(); err != nil {
			t.Fatalf("beat failed: %v", err)
		}
		now = next
	}
	t.Fatal("the loop never ended")
	return "", now
}

func TestEveryShowcasePlaysItsStory(t *testing.T) {
	want := map[kernel.PolicyID]string{
		editor.PairsPolicyID:      "The tide turned (slowly) and she said “stay.”",
		editor.WritingPolicyID:    "## Tides\n*Write* *softly*, or **firmly**.\n- keep `notes` and ~strike~ drafts",
		editor.CJKPolicyID:        "我用 Go 写作，每天写 500 字。",
		editor.TypographyPolicyID: "Wait… — it’s “almost” done.",
		editor.HistoryPolicyID:    "A first draft, then a second thought.",
		editor.NavigationPolicyID: "Leave a scope (with Tab) and keep writing.",
		editor.WordsPolicyID:      "Words move as units. Sentences do too.",
		editor.TextPolicyID:       "Every keystroke is an intent.",
	}
	source := defaultConfiguration(t)
	for policy, final := range want {
		script, ok := For(policy)
		if !ok {
			t.Fatalf("%s has no showcase", policy)
		}
		player := NewPlayer(script, Configuration(policy, source), time.Unix(1_700_000_000, 0))
		if got, _ := playLoop(t, player, time.Unix(1_700_000_000, 0)); got != final {
			t.Errorf("%s ended on %q, want %q", policy, got, final)
		}
	}
	if len(scripts) != len(want) {
		t.Fatalf("%d scripts, %d checked", len(scripts), len(want))
	}
}

func TestShowcaseLoopsAndFadesAtTheSeams(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	script, _ := For(editor.TypographyPolicyID)
	player := NewPlayer(script, defaultConfiguration(t), start)
	if opacity := player.Opacity(start); opacity != 0 {
		t.Fatalf("opacity at the start of a loop = %v, want a fade in", opacity)
	}
	if opacity := player.Opacity(start.Add(FadeIn)); opacity != 1 {
		t.Fatalf("opacity after the fade in = %v", opacity)
	}
	now := start
	for player.ended.IsZero() {
		now = player.Advance(now)
	}
	restart := player.ended.Add(script.Hold)
	if opacity := player.Opacity(restart.Add(-2 * FadeOut)); opacity != 1 {
		t.Fatalf("opacity while holding the finished page = %v", opacity)
	}
	if opacity := player.Opacity(restart.Add(-FadeOut / 2)); opacity <= 0 || opacity >= 1 {
		t.Fatalf("opacity just before the restart = %v, want a fade out", opacity)
	}
	player.Advance(restart)
	if player.Loop() != 2 || player.Document().Text() != "" {
		t.Fatalf("loop %d restarted on %q", player.Loop(), player.Document().Text())
	}
}

func TestShowcaseResumesInsteadOfRushingAfterAPause(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	script, _ := For(editor.TypographyPolicyID)
	player := NewPlayer(script, defaultConfiguration(t), start)
	now := start
	for range 6 {
		now = player.Advance(now)
	}
	typed := player.Document().Text()
	// The stage went off screen for a minute: one more beat, not a minute's.
	player.Advance(now.Add(time.Minute))
	if got := player.Document().Text(); len([]rune(got)) > len([]rune(typed))+1 {
		t.Fatalf("after a pause the page jumped from %q to %q", typed, got)
	}
	if player.Loop() != 1 {
		t.Fatal("a pause restarted the loop")
	}
}

func TestShowcaseReplaysWhenItsConfigurationChanges(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	source := defaultConfiguration(t)
	script, _ := For(editor.PairsPolicyID)
	player := NewPlayer(script, source, start)
	player.Configure(source, start)
	if player.Loop() != 1 {
		t.Fatal("an equal configuration restarted the loop")
	}
	changed := source.Clone()
	changed.SetSetting(
		kernel.NewSettingKey(editor.PairsPolicyID, kernel.NewSettingID("quotes")),
		kernel.BoolSetting(false),
	)
	player.Configure(changed, start)
	if player.Loop() != 2 {
		t.Fatal("a changed configuration did not replay the story")
	}
	if got, _ := playLoop(t, player, start); got == "The tide turned (slowly) and she said “stay.”" {
		t.Fatalf("without quote pairing the story still ended on %q", got)
	}
}
