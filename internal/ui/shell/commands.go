package shell

import (
	"strings"

	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// The app's commands are a source of the field's: a query written after
// ">" is theirs, and they answer it with a card for each group of the
// commands whose names hold every word written, each row a command and
// its key. Pressed, or Return on it, a command runs.

type commandSource struct{}

func (commandSource) answer(s *State, _ string, f *found) {
	words, ok := strings.CutPrefix(s.field.words, ">")
	if !ok {
		return
	}
	fields := strings.Fields(strings.ToLower(words))
	// What the place the query is in can have done to it stands first, as
	// the pill's end offers it.
	here := commandList{group: locales.Here()}
	for _, a := range s.placeActions() {
		label := a.label
		if a.danger {
			label += "…"
		}
		if all(fields, func(w string) bool { return strings.Contains(strings.ToLower(label), w) }) {
			here.commands = append(here.commands, Command{ID: actionCommand + a.id, Label: label, Group: here.group})
		}
	}
	if len(here.commands) > 0 {
		f.add("commands:here", tierResult, here)
	}
	if s.Commands == nil {
		return
	}
	var lists []*commandList
	at := map[string]int{}
	for _, cmd := range s.Commands() {
		label := strings.ToLower(cmd.Label)
		if !all(fields, func(w string) bool { return strings.Contains(label, w) }) {
			continue
		}
		i, ok := at[cmd.Group]
		if !ok {
			i = len(lists)
			at[cmd.Group] = i
			lists = append(lists, &commandList{group: cmd.Group})
		}
		lists[i].commands = append(lists[i].commands, cmd)
	}
	for _, l := range lists {
		f.add("commands:"+l.group, tierResult, *l)
	}
}

// actionCommand begins the ID of a command that is one of the place's
// actions.
const actionCommand = "action:"

// runActionCommand does the place's action id as the `>` commands run
// it: what asks again opens the actions with it turned, to be pressed.
func (s *State) runActionCommand(id string) {
	for i, a := range s.placeActions() {
		if a.id != id {
			continue
		}
		if !a.danger {
			a.run()
			return
		}
		s.field.acting, s.field.actionAt = true, i
		s.closeKey()
		s.arm(a.id)
		return
	}
}

func all[T any](items []T, ok func(T) bool) bool {
	for _, it := range items {
		if !ok(it) {
			return false
		}
	}
	return true
}

// commandList is a card of one group's commands.
type commandList struct {
	group    string
	commands []Command
}

func (l commandList) shape(_ *State, _ *ui.Context, m blockMetrics) shape {
	rows := make([]row, len(l.commands))
	for i := range l.commands {
		rows[i] = row{command: &l.commands[i], h: m.groupRow}
	}
	return stack(rows, m)
}

func (l commandList) rows() []string {
	ids := make([]string, len(l.commands))
	for i, cmd := range l.commands {
		ids[i] = "command:" + cmd.ID
	}
	return ids
}

func (commandList) ground() bool { return true }

func (commandList) drawsLive(*State) bool { return false }

func (l commandList) view(s *State, c *ui.Context, semantic theme.Theme, box ui.Element, _ stageFrame, sh shape, _ bool, m blockMetrics, animate bool) {
	box.Gap(m.headGap).Children(func() {
		cardHead(c, semantic, components.IconCommand, l.group, m)
		cardRows(c, sh, m, func(r row) {
			if r.command != nil {
				s.commandRow(c, semantic, *r.command, m, animate)
				return
			}
			moreLabel(c, semantic, sh.more, r.h, m)
		})
	})
}

// commandRow is one of the app's commands: pressed, it runs.
func (s *State) commandRow(c *ui.Context, semantic theme.Theme, cmd Command, m blockMetrics, animate bool) {
	id := "command:" + cmd.ID
	btn := ui.ButtonBase(c.Key("row-" + id)).WidthPercent(100).Height(m.groupRow).PaddingX(rowPad(m.scale)).
		Gap(8 * m.k).AlignItems(ui.Center).Radius(theme.DefaultControls.Scaled(m.scale).Radius).
		FocusRing(false).KeepFocus().Label(cmd.Label)
	s.rowFill(c, semantic, btn, id, animate)
	if btn.Clicked() {
		s.activate(id, false)
	}
	btn.Children(func() {
		components.CenteredLabel(c, cmd.Label, theme.TextSmall, semantic.Text.Default).Grow(1).MinWidth(0)
		if len(cmd.Keys) > 0 {
			components.Keycaps(c, components.KeycapStyle(semantic, m.scale, true), cmd.Keys, cmd.Binding).Shrink(0)
		}
	})
}
