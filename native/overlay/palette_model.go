// Package overlay owns transient command and page surfaces.
package overlay

import (
	"sort"
	"strings"

	"github.com/caelyreth/neverland/engine/kernel"
)

type Command struct {
	ID      kernel.CommandID
	Label   string
	Group   string
	Binding string
}

type CommandPalette struct {
	Open     bool
	Query    string
	Commands []Command
	Selected int
	Chosen   *kernel.CommandID
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
}

func (p *CommandPalette) Filtered() []Command {
	query := strings.ToLower(strings.TrimSpace(p.Query))
	if query == "" {
		return append([]Command(nil), p.Commands...)
	}
	result := make([]Command, 0, len(p.Commands))
	for _, command := range p.Commands {
		haystack := strings.ToLower(command.Label + " " + command.Group + " " + string(command.ID))
		if strings.Contains(haystack, query) {
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

func (p *CommandPalette) Submit() (kernel.CommandID, bool) {
	filtered := p.Filtered()
	if p.Selected < 0 || p.Selected >= len(filtered) {
		return "", false
	}
	id := filtered[p.Selected].ID
	p.Chosen = &id
	p.Open = false
	return id, true
}

func (p *CommandPalette) ConsumeChoice() (kernel.CommandID, bool) {
	if p.Chosen == nil {
		return "", false
	}
	choice := *p.Chosen
	p.Chosen = nil
	return choice, true
}
