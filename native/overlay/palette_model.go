package overlay

import (
	"sort"
	"strings"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/egoist/mygo/ui"
)

type Command struct {
	ID    keymap.CommandID
	Label string
	Group string
	// Binding is the command's shortcut as one chord, and Keys its caps.
	Binding string
	Keys    []string
}

type CommandPalette struct {
	Open     bool
	Query    string
	Commands []Command
	Selected int
	Chosen   *keymap.CommandID
	scroll   ui.ScrollState
	over     scrollfx.Overscroll
}

func (p *CommandPalette) Reset(commands []Command) {
	p.Commands = append([]Command(nil), commands...)
	sort.SliceStable(p.Commands, func(i, j int) bool {
		if p.Commands[i].Group != p.Commands[j].Group {
			return p.Commands[i].Group < p.Commands[j].Group
		}
		return p.Commands[i].Label < p.Commands[j].Label
	})
	p.Query = ""
	p.Selected = 0
	p.scroll = ui.ScrollState{}
	p.over.Reset()
}

func (p *CommandPalette) Filtered() []Command {
	query := strings.ToLower(strings.TrimSpace(p.Query))
	if query == "" {
		return append([]Command(nil), p.Commands...)
	}
	words := strings.Fields(query)
	result := make([]Command, 0, len(p.Commands))
	for _, command := range p.Commands {
		label := strings.ToLower(command.Label)
		matches := true
		for _, word := range words {
			if !strings.Contains(label, word) {
				matches = false
				break
			}
		}
		if matches {
			result = append(result, command)
		}
	}
	return result
}

func (p *CommandPalette) Move(delta int) {
	filtered := p.Filtered()
	if len(filtered) == 0 {
		p.Selected = 0
		return
	}
	p.Selected = (p.Selected + delta + len(filtered)) % len(filtered)
}

func (p *CommandPalette) Submit() (keymap.CommandID, bool) {
	filtered := p.Filtered()
	if p.Selected < 0 || p.Selected >= len(filtered) {
		return "", false
	}
	id := filtered[p.Selected].ID
	p.Chosen = &id
	p.Open = false
	return id, true
}

func (p *CommandPalette) ConsumeChoice() (keymap.CommandID, bool) {
	if p.Chosen == nil {
		return "", false
	}
	choice := *p.Chosen
	p.Chosen = nil
	return choice, true
}
