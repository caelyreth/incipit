package controller

import (
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

// Execute routes one stable command ID through the editor session. Commands
// unknown to the editor remain queued for the application workspace.
func (c *DocumentController) Execute(command Command) (kernel.Outcome, error) {
	if command.ID == "" {
		return kernel.OutcomePass, c.fail(ErrUnknownCommand)
	}
	if command.Range != nil {
		return c.Replace(command.Range, command.Text, command.Origin)
	}
	for _, next := range editor.AllTextCommands {
		if string(next.ID()) == command.ID {
			outcome, err := c.session.ExecuteText(next)
			return c.finishCommand(outcome, err)
		}
	}
	for _, next := range editor.AllHistoryCommands {
		if string(next.ID()) == command.ID {
			outcome, err := c.session.ExecuteHistory(next)
			return c.finishCommand(outcome, err)
		}
	}
	for _, next := range editor.AllWordCommands {
		if string(next.ID()) == command.ID {
			outcome, err := c.session.ExecuteWord(next)
			return c.finishCommand(outcome, err)
		}
	}
	for _, next := range []editor.MarkupCommand{
		editor.MarkupEmphasis,
		editor.MarkupStrong,
	} {
		if string(next.ID()) == command.ID {
			outcome, err := c.session.ExecuteMarkup(next)
			return c.finishCommand(outcome, err)
		}
	}
	c.pending = append(c.pending, command)
	c.setStatus(kernel.OutcomePass, false, ErrUnknownCommand)
	return kernel.OutcomePass, ErrUnknownCommand
}

// DrainCommands returns queued commands outside the editor's command set.
func (c *DocumentController) DrainCommands() []Command {
	commands := append([]Command(nil), c.pending...)
	c.pending = c.pending[:0]
	return commands
}

func (c *DocumentController) finishCommand(
	outcome kernel.Outcome,
	err error,
) (kernel.Outcome, error) {
	if err != nil {
		return kernel.OutcomePass, c.fail(err)
	}
	if outcome == kernel.OutcomeHandled {
		c.refreshSource(true)
		c.refreshVisible()
	}
	c.setStatus(outcome, outcome == kernel.OutcomeHandled, nil)
	return outcome, nil
}
