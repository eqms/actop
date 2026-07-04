# actop Container Action Menu (ctop-style) — Design

Date: 2026-07-04
Status: approved
Target version: 0.4.0

## Goal

Pressing `enter` on a container in the containers tab opens a centered
action-menu panel, modeled after ctop's per-container menu. The menu lists
only actions valid for the container's current state; destructive actions
keep the existing confirmation dialog. Direct grid hotkeys stay unchanged.

## Scope

- Containers tab only. Volumes/images tabs keep their current behavior
  (`D` + confirm). Extending the menu to other tabs is a possible follow-up,
  not part of this change.
- No new dependencies; the menu is a new screen in the existing screen
  stack (same pattern as confirm/columns/help).

## Behavior

### Trigger

- `enter` on the selected container row pushes the menu screen
  (replaces the previous behavior of opening the detail screen directly).
- The cursor starts on "Details", so `enter enter` still reaches the
  detail view quickly.

### Menu items (state-filtered)

Running container:

| Label    | Hotkey | Action                                          |
|----------|--------|--------------------------------------------------|
| Details  | enter  | push detail screen                               |
| Logs     | l      | push logs screen                                 |
| Shell    | e      | suspend TUI, exec shell                          |
| Open URL | o      | open WebURL in browser (item only if WebURL != "") |
| — separator —
| Stop     | X      | push confirm("stop")                             |
| Kill     | K      | push confirm("kill")                             |
| Delete   | D      | push confirm("delete")                           |

Stopped container:

| Label    | Hotkey | Action                  |
|----------|--------|--------------------------|
| Details  | enter  | push detail screen      |
| Logs     | l      | push logs screen        |
| — separator —
| Start    | S      | push confirm("start")   |
| Delete   | D      | push confirm("delete")  |

### Keys inside the menu

- `j`/`k`/`down`/`up`: move cursor (separator is skipped, no wrap-around
  required — clamp at ends is fine)
- `enter`: run the selected item
- item hotkey (`l`, `e`, `o`, `S`, `X`, `K`, `D`): run that item directly
- `esc`/`q`: close the menu, return to the grid

### Rendering

Centered bordered panel, visually consistent with the existing confirm
screen. Title is the container name. Each row shows label and hotkey;
the selected row is highlighted with a cursor marker. Footer hint:
`↑/↓ select · enter · esc`.

```
┌─ picard-dev-db-19-native ──────┐
│  ▸ Details          enter      │
│    Logs             l          │
│    Shell            e          │
│    Open URL         o          │
│    ──────────────────────      │
│    Stop             X          │
│    Kill             K          │
│    Delete           D          │
│                                │
│  ↑/↓ wählen · enter · esc      │
└────────────────────────────────┘
```

## Components

- **New** `ui/menu.go`: `menuScreen` holding the container ID, name and a
  `[]menuItem{label, hotkey string, cmd func() (screen, tea.Cmd)}` built
  from the container state at open time. Implements the existing `screen`
  interface (`update`, `view`).
- **Changed** `ui/grid.go`: `enter` case pushes `newMenuScreen(app, c)`
  instead of `newDetailScreen`. Direct hotkeys on the grid stay as-is.
- **Changed** `ui/help.go`: key list gains the menu entry
  (`enter — action menu`).
- Reuse: `confirmAction` (ui/confirm.go), `execShellCmd`/`openURLCmd`
  (ui/commands.go), panel styles (ui/styles.go).

## Error handling

The menu captures only the container ID/state snapshot at open time. If
the container disappears before an action runs, the existing error paths
apply (`CLIError`/`ErrNotFound` → status banner). No new error channel.

Actions selected from the menu replace the menu screen (menu closes, the
follow-up screen — detail/logs/confirm — is pushed, or the command runs).

## Testing

Table-driven tests following the existing `ui/app_test.go` patterns:

- `enter` on grid opens the menu; title shows container name
- running container: full item list; stopped container: reduced list
- Open URL item present only when `WebURL()` is non-empty
- cursor navigation clamps at ends and skips the separator
- `enter` on "Details" pushes detail screen
- selecting Stop/Kill/Delete/Start pushes the confirm screen (no direct
  execution)
- hotkey inside menu selects the matching item
- `esc` closes the menu without action

## Release

- Version 0.4.0 (`VERSION`), bilingual entry in `RELEASE_NOTES.md`
- Commit prefix `[ADD]`
