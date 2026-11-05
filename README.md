# mdnano

A lightweight terminal-based Markdown reader and editor featuring a file sidebar, written in Go using the Charm ecosystem (`Bubble Tea`, `Glamour`, `Bubbles`).

## Features

* **Dual-Panel Interface:** Sidebar listing `.md` files alongside a reading and editing area.
* **Fast Search & Navigation:** Built-in fuzzy filtering for documents via the `/` key.
* **Editing Mode:** Full multi-line text editor with line numbers.
* **Markdown Cheatsheet:** Built-in syntax reference accessible via `?` or `F1`.
* **High Performance:** Optimized rendering cache and no blocking terminal theme queries.

## Installation & Running

1. Save the source code to a file named `main.go` (or `mdnano.go`).
2. Initialize the module and install dependencies:
```bash
go mod init mdnano
go mod tidy

```


3. Run the application (specifying the target directory containing `.md` files):
```bash
go run main.go .

```



## Keyboard Shortcuts

| Key | Action |
| --- | --- |
| `Tab` | Switch focus between the file list and main area |
| `↑` / `↓` | Navigate the file list or scroll text |
| `/` | Search/filter files in the list |
| `n` | Create a new Markdown document |
| `e` | Switch to editing mode for the selected file |
| `Ctrl + S` | Save file changes in editor mode |
| `?` / `h` | Open the Markdown cheatsheet |
| `b` | Toggle the sidebar on/off |
| `Esc` | Exit editor/cheatsheet mode or cancel |
| `q` / `Ctrl + C` | Quit the application |

## Dependencies

* [`bubbletea`](https://github.com/charmbracelet/bubbletea) — TUI framework
* [`glamour`](https://github.com/charmbracelet/glamour) — Markdown rendering in the terminal
* [`bubbles`](https://github.com/charmbracelet/bubbles) — Reusable components (viewport, list, textarea, textinput)
* [`lipgloss`](https://github.com/charmbracelet/lipgloss) — Styling and layout toolkit
