package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/eqms/actop/cli"
)

// imagesScreen is the images tab: a read-only list with delete action.
type imagesScreen struct {
	app    *App
	cursor int
}

func newImagesScreen(app *App) *imagesScreen {
	return &imagesScreen{app: app}
}

func (s *imagesScreen) rows() []cli.Image {
	imgs := make([]cli.Image, len(s.app.images))
	copy(imgs, s.app.images)
	sort.Slice(imgs, func(i, j int) bool {
		return imgs[i].Configuration.Name < imgs[j].Configuration.Name
	})
	return imgs
}

func (s *imagesScreen) selected() *cli.Image {
	rows := s.rows()
	if len(rows) == 0 {
		return nil
	}
	if s.cursor >= len(rows) {
		s.cursor = len(rows) - 1
	}
	return &rows[s.cursor]
}

func (s *imagesScreen) update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "q", "ctrl+c":
		return s, tea.Quit
	case "j", "down":
		if s.cursor < len(s.rows())-1 {
			s.cursor++
		}
	case "k", "up":
		if s.cursor > 0 {
			s.cursor--
		}
	case "h", "?":
		return s, s.app.push(newHelpScreen())
	case "D":
		if img := s.selected(); img != nil {
			ref := img.Configuration.Name
			q := fmt.Sprintf("delete image %q?", ref)
			return s, s.app.push(newConfirmScreen(q, imageDeleteCmd(s.app.client, ref)))
		}
	}
	return s, nil
}

func (s *imagesScreen) view(width, height int) string {
	rows := s.rows()

	refWidth := 12
	for _, img := range rows {
		if len(img.Configuration.Name) > refWidth {
			refWidth = len(img.Configuration.Name)
		}
	}

	var b strings.Builder
	head := []string{pad("REFERENCE", refWidth), pad("DIGEST", 12), pad("CREATED", 17), "VARIANTS"}
	b.WriteString(styleColHead.Render(strings.Join(head, "  ")))
	b.WriteString("\n")

	for i, img := range rows {
		cells := []string{
			pad(img.Configuration.Name, refWidth),
			pad(img.ShortDigest(), 12),
			pad(img.Configuration.CreationDate.Local().Format("2006-01-02 15:04"), 17),
			fmt.Sprintf("%d", len(img.Variants)),
		}
		line := strings.Join(cells, "  ")
		if i == s.cursor {
			line = styleSelected.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	if len(rows) == 0 {
		b.WriteString(styleDim.Render("no images"))
		b.WriteString("\n")
	}
	return b.String()
}
