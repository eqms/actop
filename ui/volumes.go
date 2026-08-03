package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/eqms/actop/cli"
)

// volumesScreen is the volumes tab: a read-only list with delete action.
type volumesScreen struct {
	app    *App
	cursor int
}

func newVolumesScreen(app *App) *volumesScreen {
	return &volumesScreen{app: app}
}

func (v *volumesScreen) rows() []cli.Volume {
	vols := make([]cli.Volume, len(v.app.volumes))
	copy(vols, v.app.volumes)
	sort.Slice(vols, func(i, j int) bool {
		return vols[i].Configuration.Name < vols[j].Configuration.Name
	})
	return vols
}

func (v *volumesScreen) selected() *cli.Volume {
	rows := v.rows()
	if len(rows) == 0 {
		return nil
	}
	if v.cursor >= len(rows) {
		v.cursor = len(rows) - 1
	}
	return &rows[v.cursor]
}

func (v *volumesScreen) update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return v, nil
	}
	switch key.String() {
	case "q", "ctrl+c":
		return v, tea.Quit
	case "j", "down":
		if v.cursor < len(v.rows())-1 {
			v.cursor++
		}
	case "k", "up":
		if v.cursor > 0 {
			v.cursor--
		}
	case "h", "?":
		return v, v.app.push(newHelpScreen())
	case "enter":
		if vol := v.selected(); vol != nil {
			return v, v.app.push(newVolumeMenu(v.app, vol.Configuration.Name))
		}
	case "D":
		if vol := v.selected(); vol != nil {
			return v, v.app.push(volumeDeleteConfirm(v.app, vol.Configuration.Name))
		}
	}
	return v, nil
}

// volumeDeleteConfirm is shared by the D hotkey and the enter menu.
func volumeDeleteConfirm(app *App, name string) *confirmScreen {
	q := fmt.Sprintf("delete volume %q? (data is lost)", safe(name))
	return newConfirmScreen(q, volumeDeleteCmd(app.client, name))
}

func (v *volumesScreen) view(width, height int) string {
	rows := v.rows()

	nameWidth := 12
	for _, vol := range rows {
		if w := displayWidth(vol.Configuration.Name); w > nameWidth {
			nameWidth = w
		}
	}

	var b strings.Builder
	head := []string{pad("NAME", nameWidth), pad("FORMAT", 7), pad("SIZE", 8), pad("CREATED", 17), "SOURCE"}
	b.WriteString(styleColHead.Render(strings.Join(head, "  ")))
	b.WriteString("\n")

	for i, vol := range rows {
		c := vol.Configuration
		cells := []string{
			pad(c.Name, nameWidth),
			pad(c.Format, 7),
			pad(byteSize(c.SizeInBytes), 8),
			pad(c.CreationDate.Local().Format("2006-01-02 15:04"), 17),
			safe(c.Source),
		}
		line := strings.Join(cells, "  ")
		if i == v.cursor {
			line = styleSelected.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	if len(rows) == 0 {
		b.WriteString(styleDim.Render("no volumes"))
		b.WriteString("\n")
	}
	return b.String()
}
