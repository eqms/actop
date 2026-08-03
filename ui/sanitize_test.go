package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/eqms/actop/cli"
)

// evil is a name/reference an image publisher can choose freely. It embeds
// an OSC 52 clipboard write, a window-title set and an SGR color switch --
// the exact sequences redact.Line exists to strip before rendering.
const evil = "web\x1b]52;c;aGFja2Vk\x07\x1b]0;pwned\x07\x1b[31mX"

// evilMarkers are fragments that must never survive into a rendered view.
// Checking payload text (not just the escape bytes) catches a half-fix that
// drops \x1b but leaves the OSC body on screen.
var evilMarkers = []string{"\x1b]", "\x07", "52;c;", "aGFja2Vk", "pwned"}

func assertNoEscapes(t *testing.T, what, out string) {
	t.Helper()
	for _, m := range evilMarkers {
		if strings.Contains(out, m) {
			t.Errorf("%s renders unsanitized terminal sequence %q", what, m)
		}
	}
}

// evilApp returns an app whose every CLI-supplied string carries the payload.
func evilApp(t *testing.T) *App {
	t.Helper()
	app, _ := testApp()
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app.Update(listResultMsg{items: []cli.ListItem{{
		ID: evil,
		Configuration: cli.Configuration{
			Image:        cli.ImageRef{Reference: evil},
			CreationDate: time.Now(),
			InitProcess: cli.InitProcess{
				Environment: []string{
					"HOME=" + evil,
					evil + "=plain",
					evil, // no "=" at all
				},
			},
		},
		Status: &cli.Status{
			State:    evil,
			Networks: []cli.NetworkAttachment{{IPv4Address: evil}},
		},
	}}})
	app.Update(volumesResultMsg{volumes: []cli.Volume{{
		ID: evil,
		Configuration: cli.VolumeConfig{
			Name: evil, Format: evil, Source: evil,
		},
	}}})
	app.Update(imagesResultMsg{images: []cli.Image{{
		ID: evil,
		Configuration: cli.ImageConfig{
			Name:       evil,
			Descriptor: cli.Descriptor{Digest: "sha256:" + evil},
		},
	}}})
	app.Update(systemStatusMsg{status: &cli.SystemStatus{
		Status: evil, APIServerVersion: evil, AppRoot: evil,
	}})
	return app
}

func TestTabsStripTerminalEscapes(t *testing.T) {
	app := evilApp(t)
	for i, name := range []string{"containers", "volumes", "images", "system"} {
		app.switchTab(i)
		assertNoEscapes(t, name+" tab", app.Body())
	}
}

func TestDetailScreenStripsTerminalEscapes(t *testing.T) {
	app := evilApp(t)
	d := newDetailScreen(app, evil)
	assertNoEscapes(t, "detail screen", d.view(120, 40))
}

func TestMenuAndConfirmStripTerminalEscapes(t *testing.T) {
	app := evilApp(t)

	c, ok := app.store.Get(evil)
	if !ok {
		t.Fatal("container not in store")
	}
	assertNoEscapes(t, "container menu", newContainerMenu(app, c).view(120, 40))
	assertNoEscapes(t, "volume menu", newVolumeMenu(app, evil).view(120, 40))
	assertNoEscapes(t, "image menu", newImageMenu(app, evil).view(120, 40))

	assertNoEscapes(t, "delete confirm", confirmAction(app, "delete", evil).view(120, 40))
	assertNoEscapes(t, "volume delete confirm", volumeDeleteConfirm(app, evil).view(120, 40))
	assertNoEscapes(t, "image delete confirm", imageDeleteConfirm(app, evil).view(120, 40))
}

func TestLogsScreenStripsTerminalEscapes(t *testing.T) {
	app := evilApp(t)
	l := newLogsScreen(app, evil)
	l.update(logLineMsg{line: evil})
	assertNoEscapes(t, "logs screen", l.view(120, 40))
}

// Column widths are derived from the strings they hold. Measuring the raw
// value would reserve cells for escape bytes that are never drawn, so the
// width must be taken after sanitizing -- and counted in runes, not bytes.
func TestDisplayWidthIgnoresEscapesAndCountsRunes(t *testing.T) {
	if got := displayWidth(evil); got != 4 { // "webX"
		t.Errorf("displayWidth(evil) = %d, want 4", got)
	}
	if got := displayWidth("grün"); got != 4 {
		t.Errorf("displayWidth(\"grün\") = %d, want 4 (runes, not bytes)", got)
	}
}

// open(1) launches applications and files, not just browsers, so anything
// that is not an http(s) URL must be refused before it reaches the process.
func TestOpenURLRefusesNonHTTPSchemes(t *testing.T) {
	for _, raw := range []string{
		"/Applications/Calculator.app",
		"file:///etc/passwd",
		"ftp://host/x",
		"",
		"://nonsense",
	} {
		msg := openURLCmd(raw)()
		done, ok := msg.(actionDoneMsg)
		if !ok {
			t.Fatalf("openURLCmd(%q) returned %T", raw, msg)
		}
		if done.err == nil {
			t.Errorf("openURLCmd(%q) accepted a non-http target", raw)
		}
	}
}

func TestPadHandlesNonPositiveWidth(t *testing.T) {
	if got := pad("anything", 0); got != "" {
		t.Errorf("pad(_, 0) = %q, want empty", got)
	}
	if got := pad("anything", -1); got != "" {
		t.Errorf("pad(_, -1) = %q, want empty", got)
	}
}
