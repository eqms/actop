# actop Menu v2 (ctop-faithful, all tabs) — Design

Date: 2026-07-05
Status: approved
Target version: 0.5.0
Supersedes the menu behavior of `2026-07-04-actop-menu-design.md`.

## Goal

Bring the action menu closer to ctop's original structure — fixed item list
with disabled (grayed-out) entries instead of state-filtering, plain label
list without hotkey column/separator/footer, explicit Cancel entry — and
extend the enter-menu uniformly to the volumes and images tabs.

Feedback basis: the Captain prefers ctop fidelity over "improved"
alternatives (see memory `actop-ux-ctop-fidelity`).

## Behavior

### Container menu (containers tab, `enter`)

Fixed order, always all entries:

| Label    | Enabled when                       | Action                    |
|----------|------------------------------------|---------------------------|
| Details  | always                             | push detail screen        |
| Logs     | always                             | push logs screen          |
| Shell    | running                            | exec shell (menu closes)  |
| Open URL | running AND WebURL() != ""         | open browser (menu closes)|
| Start    | NOT running                        | confirm("start")          |
| Stop     | running                            | confirm("stop")           |
| Kill     | running                            | confirm("kill")           |
| Delete   | always                             | confirm("delete")         |
| Cancel   | always                             | close menu                |

- Disabled entries render dimmed (styleDim) and are skipped by cursor
  movement; they can never be selected.
- Cursor starts on the first enabled entry (always "Details").

### Volumes menu (volumes tab, `enter` on a volume)

| Label  | Enabled | Action                                        |
|--------|---------|-----------------------------------------------|
| Delete | always  | existing volume-delete confirm (data-loss warning) |
| Cancel | always  | close menu                                    |

Title: volume name.

### Images menu (images tab, `enter` on an image)

| Label  | Enabled | Action                          |
|--------|---------|---------------------------------|
| Delete | always  | existing image-delete confirm   |
| Cancel | always  | close menu                      |

Title: image reference.

### Keys inside any menu

- `j`/`k`/`down`/`up`: move cursor, skipping disabled entries, clamping at
  the ends
- `enter`: run the selected entry
- `esc`/`q`: close the menu
- No per-item hotkeys inside the menu (removed). Direct hotkeys on the grid
  and on the volumes/images tabs (`l`, `e`, `o`, `S`, `X`, `K`, `D`) stay
  unchanged.

### Rendering (all menus)

Centered bordered panel (existing styleMenuPanel), title line, one label
per row, selected row marked with `▸ ` + styleSelected, disabled rows
rendered with styleDim. No hotkey column, no separator line, no footer
hint.

## Components

- `ui/menu.go`:
  - `menuItem{label string; disabled bool; build func(*menuScreen) (screen, tea.Cmd)}`
    — `disabled` replaces the nil-build separator concept; Cancel's build
    returns `(nil, nil)`.
  - `menuScreen{app, title string, items, cursor}` — generic; `id` field
    replaced by `title` (actions capture what they need in closures).
  - `newContainerMenu(app *App, c *model.Container) *menuScreen` (renamed
    from `newMenuScreen`, builds the fixed list with enabled flags),
    `newVolumeMenu(app *App, name string) *menuScreen`,
    `newImageMenu(app *App, ref string) *menuScreen`.
  - `move()` skips `disabled` items; constructor places the cursor on the
    first enabled item.
  - `update()`: remove the hotkey-dispatch loop; `enter` ignores disabled
    selection (cannot occur by construction, guard defensively).
  - `view()`: drop hotkey column, separator, footer.
- `ui/grid.go`: `enter` pushes `newContainerMenu`.
- `ui/volumes.go`, `ui/images.go`: new `enter` case pushes the respective
  menu (existing `D` handling stays).
- `ui/help.go`: `enter` entry covers all tabs ("action menu (per tab)").
- Existing volume/image delete confirms are reused
  (`ui/volumes.go`/`ui/images.go` already build them for the `D` key —
  extract/reuse, do not duplicate the warning text).

## Error handling

Unchanged from v1: menus capture a snapshot at open time; failures of
actions surface via the existing CLIError → status banner path.

## Testing

Rewrite/extend `ui/menu_test.go`:

- container menu running: all 9 entries present; Shell/Open URL/Stop/Kill
  enabled, Start disabled (with WebURL: Open URL enabled; without: disabled)
- container menu stopped: Start/Delete/Details/Logs/Cancel enabled;
  Shell/Open URL/Stop/Kill disabled
- cursor starts on Details; movement skips disabled entries and clamps
- enter on Details → detail screen; enter on Stop → confirm; Cancel and
  esc close (nil, nil)
- no hotkey dispatch inside the menu (pressing 'X' inside the menu does NOT
  open a confirm)
- view: labels rendered, disabled entries dimmed vs. enabled (presence
  check), marker on selected line, no footer text
- volumes tab: enter opens volume menu; Delete → confirm → mock
  VolumeDelete call; Cancel closes
- images tab: enter opens image menu; Delete → confirm → mock ImageDelete
- grid/volumes/images direct hotkeys unchanged (existing tests keep
  passing)

## Release

- VERSION 0.5.0, bilingual RELEASE_NOTES entry.
- README: keys table (`enter` = action menu in all tabs), "Neu in
  v0.5"/"New in v0.5" section; the roadmap heading "Roadmap (v0.5)" is
  retitled to "Roadmap (v0.6)" with its three items unchanged (none of
  them ship in 0.5.0).
- Commit prefix `[CHG]` (behavior change) / `[ADD]` for the new tab menus —
  final release commit `[ADD] v0.5.0: ...`.
