package page

import (
	"github.com/caelyreth/neverland/editor/present"
	"github.com/egoist/mygo/ui"
)

func (el *Element) fieldValue() string {
	if el.Editor == nil || el.Editor.layout == nil {
		return ""
	}
	pc, ok := el.Editor.layout.lookup(el.Editor.Selection().Head)
	if !ok || pc.surface == nil {
		return ""
	}
	return pc.surface.text
}

// accessUnits projects each presentation unit as a positioned node in the
// page's accessibility tree. The boxes draw nothing and take no input;
// their roles, names, values and bounds let assistive technology move
// through the document by its units while the page's text client remains
// its one editing surface.
func (el *Element) accessUnits(c *ui.Context) {
	pl := el.Editor.layout
	if pl == nil {
		return
	}
	top := el.Top - el.offset()
	for _, u := range pl.units {
		p := u.pres
		if u.accessKey == "" {
			u.accessKey = "page-unit-" + p.Unit.String()
		}
		role, description := accessRole(p)
		width := max(el.Editor.metrics.Width-u.x, 1)
		height := max(u.h, 1)
		node := ui.Box(c).Key(u.accessKey).Absolute().
			Left(el.column + u.x).Top(top + u.y).Width(width).Height(height).
			Role(role).Label(p.Label()).Value(p.Value()).RoleDescription(description).
			IgnoreInput(true)
		if p.Level > 0 {
			node.Level(p.Level)
		}
	}
}

// accessRole is a presentation role's platform role and the app's name
// for it where the platform's is broader.
func accessRole(p *present.Presentation) (ui.Role, string) {
	switch p.Role {
	case "heading":
		return ui.RoleHeading, ""
	case "listitem":
		return ui.RoleListItem, ""
	case "code":
		return ui.RoleText, "code"
	case "math":
		return ui.RoleText, "math"
	case "separator":
		return ui.RoleSeparator, ""
	case "figure":
		return ui.RoleImage, ""
	case "table":
		return ui.RoleTable, ""
	case "cell":
		return ui.RoleCell, ""
	default:
		return ui.RoleText, ""
	}
}

// toPage is an element point in page coordinates.
