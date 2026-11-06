package controller

import (
	"github.com/caelyreth/neverland/app/keymap"
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
	action, ok := keymap.ResolveCommand(kernel.CommandID(command.ID))
	if ok && action.Owner == editor.CommandOwnerEditor {
		return c.ExecuteAction(action)
	}
	c.pending = append(c.pending, command)
	c.setStatus(kernel.OutcomePass, false, ErrUnknownCommand)
	return kernel.OutcomePass, ErrUnknownCommand
}

// ExecuteAction runs one already-resolved editor action: the intent it
// carries, through the session. Moving by a drawn row needs the page's
// geometry, which the controller holds.
func (c *DocumentController) ExecuteAction(action keymap.Action) (kernel.Outcome, error) {
	if c == nil || c.session == nil {
		return kernel.OutcomePass, keymap.ErrCommandUnavailable
	}
	if action.Owner != editor.CommandOwnerEditor || action.Kind != keymap.ActionEdit {
		return kernel.OutcomePass, keymap.ErrUnsupportedCommand
	}
	return c.Do(action.Intent)
}

// Do resolves one intent through the session.
func (c *DocumentController) Do(intent editor.Intent) (kernel.Outcome, error) {
	if intent.Kind == editor.IntentMove && intent.By == editor.ByRow {
		direction := VerticalDown
		if intent.Dir == editor.Backward {
			direction = VerticalUp
		}
		if err := c.MoveVertical(direction, intent.Extend); err != nil {
			return kernel.OutcomePass, err
		}
		return kernel.OutcomeHandled, nil
	}
	if intent.Kind != editor.IntentFormat && intent.Kind != editor.IntentNest {
		c.resetPreferredColumn()
	}
	result, err := c.session.Do(intent, c.now())
	return c.finishCommand(result.Outcome, err)
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
	changed := c.hasVersion && c.version.SourceRevision() != c.session.Runtime().Revision()
	if outcome == kernel.OutcomeHandled {
		c.refreshSource()
		c.refreshVisible()
	}
	c.setStatus(outcome, changed, nil)
	return outcome, nil
}
