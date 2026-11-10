# mdnano

A lightweight terminal-based Markdown reader and editor featuring a file sidebar, written in Go using the Charm ecosystem (`Bubble Tea`, `Glamour`, `Bubbles`).

## Features

* **Dual-Panel Interface:** Sidebar listing `.md` files across nested subdirectories alongside a reading and editing area.
* **Recursive File & Folder Management:** Create files in existing or new nested directories (`n`) with interactive folder autocomplete (`Tab`).
* **Distraction-Free Editing & Live Preview:** Full-width multi-line editor with active line highlighting, line numbers, automatic sidebar collapsing, and toggleable live Split-Preview (`F2`).
* **Slash Snippets & Context Hints:** Pop-up `/`-command menu for quick template insertion (tables, code blocks, checklists), hotkeys (`Ctrl+B`, `Ctrl+T`, `Ctrl+K`), and a context-aware syntax inspector bar.
* **Fast Search & Cheatsheet:** Built-in fuzzy file filtering (`/`) and an interactive Markdown syntax reference (`?` or `F1`).
* **High Performance:** Instant navigation powered by an in-memory rendering cache and non-blocking terminal theme detection.






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

**Navigation & View Mode**

| Key | Action |
| --- | --- |
| `Tab` | Switch focus between the file list and reading area|
| `↑` / `↓` | Navigate the file list or scroll document text|
| `Enter` | Open the selected file and focus the reader|
| `/` | Fuzzy search/filter files in the sidebar|
| `n` | Open the new file & directory creation dialog|
| `e` | Edit the selected file (auto-collapses sidebar)|
| `?` / `h` | Open the Markdown syntax cheatsheet|
| `b` | Toggle the file sidebar on/off|
| `q` / `Ctrl + C` | Quit the application|

**Editor Mode**

| Key | Action |
| --- | --- |
| `/` | Open the pop-up snippet menu (`/table`, `/code`, `/todo`, etc.) |
| `F2` | Toggle live Split-Preview on/off |
| `Ctrl + B` | Insert bold text (`**жирный**`) |
| `Ctrl + T` | Insert a Markdown table template |
| `Ctrl + K` | Insert a link template (`[текст](https://)`) |
| `F1` | Open the Markdown cheatsheet from the editor|
| `Ctrl + S` | Save changes and return to view mode|
| `Esc` | Cancel editing or close pop-up menu|

**New File Dialog (`n`)**

| Key | Action |
| --- | --- |
| `↑` / `↓` | Select an existing folder from the suggestions list |
| `Tab` | Autocomplete the selected folder path into the input field |
| `Enter` | Create the file (and any missing nested folders) and open editor|
| `Esc` | Cancel file creation|
## Dependencies

* [`bubbletea`](https://github.com/charmbracelet/bubbletea) — TUI framework
* [`glamour`](https://github.com/charmbracelet/glamour) — Markdown rendering in the terminal
* [`bubbles`](https://github.com/charmbracelet/bubbles) — Reusable components (viewport, list, textarea, textinput)
* [`lipgloss`](https://github.com/charmbracelet/lipgloss) — Styling and layout toolkit
