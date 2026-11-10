package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type focusState int

const (
	focusList focusState = iota
	focusReader
)

type rightPaneMode int

const (
	modeView rightPaneMode = iota
	modeEdit
	modeHelp
	modeNewFile
)

const sidebarWidth = 30

type snippet struct {
	trigger string
	label   string
	insert  string
}

var markdownSnippets = []snippet{
	{"/h1", "Заголовок 1", "# "},
	{"/h2", "Заголовок 2", "## "},
	{"/h3", "Заголовок 3", "### "},
	{"/bold", "Жирный текст", "**жирный текст**"},
	{"/italic", "Курсив", "*курсив*"},
	{"/code", "Блок кода (Go)", "```go\n\n```"},
	{"/sh", "Блок кода (Bash)", "```bash\n\n```"},
	{"/table", "Таблица 3x2", "| Заголовок 1 | Заголовок 2 | Заголовок 3 |\n| :--- | :---: | ---: |\n| Ячейка 1 | Ячейка 2 | Ячейка 3 |\n| Ячейка 4 | Ячейка 5 | Ячейка 6 |"},
	{"/todo", "Список задач", "- [ ] Задача 1\n- [ ] Задача 2"},
	{"/quote", "Цитата", "> "},
	{"/link", "Ссылка", "[Текст ссылки](https://example.com)"},
	{"/hr", "Разделитель", "\n---\n"},
}

const markdownCheatsheet = `# Шпаргалка по Markdown

## Быстрые команды в редакторе
Нажмите **/** в редакторе для вызова всплывающего меню сниппетов, или используйте хоткеи:
* **F2** — включить/выключить синхронный предпросмотр (Split Preview)
* **Ctrl+B** — вставить **жирный** текст
* **Ctrl+T** — вставить шаблон таблицы
* **Ctrl+K** — вставить ссылку

## Синтаксис таблиц
` + "```markdown" + `
| Левый край | По центру | Правый край |
| :--- | :---: | ---: |
| Текст | Текст | 100 $ |
` + "```" + `
`

var (
	activeBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))

	editBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("214"))

	helpBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("42"))

	newFileBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("39"))

	inactiveBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	popupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("39")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	popupSelected = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Bold(true)

	popupNormal = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	hintBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("237")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			PaddingLeft(1)

	msgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)

	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)
)

type fileItem struct {
	name string
	path string
	info string
}

func (i fileItem) Title() string       { return i.name }
func (i fileItem) Description() string { return i.info }
func (i fileItem) FilterValue() string { return i.name }

type model struct {
	list              list.Model
	viewport          viewport.Model
	textarea          textarea.Model
	nameInput         textinput.Model
	renderer          *glamour.TermRenderer
	renderCache       map[string]string
	styleName         string
	lastWrapWidth     int
	targetDir         string
	focus             focusState
	mode              rightPaneMode
	showSidebar       bool
	sidebarBeforeEdit bool
	splitPreview      bool

	popupActive  bool
	popupFilter  string
	popupMatches []snippet
	popupIndex   int

	allDirs     []string
	filteredDir []string
	dirCursor   int

	rawMD       string
	currentFile string
	statusMsg   string
	width       int
	height      int
	ready       bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m *model) enterEditMode(status string) {
	m.sidebarBeforeEdit = m.showSidebar
	m.showSidebar = false
	m.mode = modeEdit
	m.focus = focusReader
	m.textarea.SetValue(m.rawMD)
	m.textarea.Focus()
	m.statusMsg = status
	m.updateLayout()
}

func (m *model) exitEditMode(targetMode rightPaneMode, status string) {
	m.popupActive = false
	m.mode = targetMode
	m.showSidebar = m.sidebarBeforeEdit
	m.textarea.Blur()
	m.statusMsg = status
	m.updateLayout()
}

func (m *model) ensureRenderer(width int) {
	wrapWidth := width - 4
	if wrapWidth < 20 {
		wrapWidth = 20
	}

	if m.renderer != nil && m.lastWrapWidth == wrapWidth {
		return
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(m.styleName),
		glamour.WithWordWrap(wrapWidth),
	)
	if err == nil {
		m.renderer = r
		m.lastWrapWidth = wrapWidth
		m.renderCache = make(map[string]string)
	}
}

func (m *model) updateLayout() {
	if !m.ready {
		return
	}

	footerHeight := 1
	contentHeight := m.height - footerHeight - 2
	if contentHeight < 5 {
		contentHeight = 5
	}

	m.list.SetSize(sidebarWidth-2, contentHeight)

	rightAreaWidth := m.width - 2
	if m.showSidebar {
		rightAreaWidth = m.width - sidebarWidth - 2
	}
	if rightAreaWidth < 20 {
		rightAreaWidth = 20
	}

	if m.mode == modeEdit && m.splitPreview {
		halfWidth := (rightAreaWidth / 2) - 1
		if halfWidth < 15 {
			halfWidth = 15
		}
		m.textarea.SetWidth(halfWidth)
		m.textarea.SetHeight(contentHeight - 2)
		m.viewport.Width = rightAreaWidth - halfWidth - 2
		m.viewport.Height = contentHeight
		m.ensureRenderer(m.viewport.Width)
	} else {
		m.viewport.Width = rightAreaWidth
		m.viewport.Height = contentHeight
		m.textarea.SetWidth(rightAreaWidth)
		m.textarea.SetHeight(contentHeight - 2)
		m.ensureRenderer(rightAreaWidth)
	}

	m.nameInput.Width = rightAreaWidth - 12
	m.refreshViewportContent()
}

func (m *model) refreshViewportContent() {
	if m.renderer == nil {
		return
	}

	if m.mode == modeHelp {
		if cached, ok := m.renderCache["__help__"]; ok {
			m.viewport.SetContent(cached)
			return
		}
		if rendered, err := m.renderer.Render(markdownCheatsheet); err == nil {
			m.renderCache["__help__"] = rendered
			m.viewport.SetContent(rendered)
		}
		return
	}

	if m.mode == modeEdit && m.splitPreview {
		liveText := m.textarea.Value()
		if liveText == "" {
			liveText = "*Начните печатать...*"
		}
		if rendered, err := m.renderer.Render(liveText); err == nil {
			m.viewport.SetContent(rendered)
		}
		return
	}

	if m.rawMD == "" {
		emptyMsg := "*Файлов пока нет или файл пуст. Нажмите **n**, чтобы создать новый документ.*"
		if rendered, err := m.renderer.Render(emptyMsg); err == nil {
			m.viewport.SetContent(rendered)
		}
		return
	}

	if cached, ok := m.renderCache[m.currentFile]; ok {
		m.viewport.SetContent(cached)
		return
	}

	if rendered, err := m.renderer.Render(m.rawMD); err == nil {
		m.renderCache[m.currentFile] = rendered
		m.viewport.SetContent(rendered)
	}
}

func (m *model) updatePopupMatches() {
	m.popupMatches = nil
	q := strings.ToLower(m.popupFilter)
	for _, s := range markdownSnippets {
		if strings.HasPrefix(s.trigger, q) || strings.Contains(strings.ToLower(s.label), strings.TrimPrefix(q, "/")) {
			m.popupMatches = append(m.popupMatches, s)
		}
	}
	if len(m.popupMatches) == 0 {
		m.popupActive = false
		m.popupFilter = ""
		m.popupIndex = 0
	} else if m.popupIndex >= len(m.popupMatches) {
		m.popupIndex = 0
	}
}

func (m *model) applySelectedSnippet() {
	if len(m.popupMatches) == 0 {
		return
	}
	chosen := m.popupMatches[m.popupIndex]

	for range []rune(m.popupFilter) {
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	m.textarea.InsertString(chosen.insert)

	m.popupActive = false
	m.popupFilter = ""
	m.popupIndex = 0
	if m.splitPreview {
		m.refreshViewportContent()
	}
}

func (m *model) updateDirSuggestions() {
	input := filepath.ToSlash(strings.TrimSpace(m.nameInput.Value()))
	dirPart := ""
	if idx := strings.LastIndex(input, "/"); idx != -1 {
		dirPart = strings.ToLower(input[:idx+1])
	} else {
		dirPart = strings.ToLower(input)
	}

	m.filteredDir = nil
	for _, d := range m.allDirs {
		if dirPart == "" || strings.Contains(strings.ToLower(d), dirPart) {
			m.filteredDir = append(m.filteredDir, d)
		}
	}
	if m.dirCursor >= len(m.filteredDir) {
		m.dirCursor = 0
	}
}

func (m *model) applySelectedDir() {
	if len(m.filteredDir) == 0 {
		return
	}
	selectedDir := m.filteredDir[m.dirCursor]
	currentVal := filepath.ToSlash(m.nameInput.Value())

	fileNamePart := ""
	if idx := strings.LastIndex(currentVal, "/"); idx != -1 {
		fileNamePart = currentVal[idx+1:]
	}

	var newVal string
	if selectedDir == "." {
		newVal = fileNamePart
	} else {
		newVal = selectedDir + "/" + fileNamePart
	}

	m.nameInput.SetValue(newVal)
	m.nameInput.CursorEnd()
	m.updateDirSuggestions()
}

func (m model) getContextHint() string {
	val := m.textarea.Value()
	lines := strings.Split(val, "\n")
	row := m.textarea.Line()
	if row >= 0 && row < len(lines) {
		curLine := strings.TrimSpace(lines[row])
		switch {
		case strings.HasPrefix(curLine, "|"):
			return "Таблица: используйте | для колонок • :--- (влево) • :---: (центр) • ---: (вправо)"
		case strings.HasPrefix(curLine, "```"):
			return "Блок кода: укажите язык после ``` (go, bash, json, python) и закройте тремя ```"
		case strings.HasPrefix(curLine, "#"):
			return "Заголовок: от # (H1) до ###### (H6) • ставьте пробел после решетки"
		case strings.HasPrefix(curLine, "- ["):
			return "Чекбокс: - [ ] для открытой задачи • - [x] для выполненной"
		case strings.HasPrefix(curLine, ">"):
			return "Цитата: > в начале строки • >> для вложенной цитаты"
		}
	}
	return "Подсказка: введите / для меню вставки • F2: превью • Ctrl+B: жирный • Ctrl+T: таблица • Ctrl+K: ссылка"
}

func (m *model) loadSelectedFile() {
	item, ok := m.list.SelectedItem().(fileItem)
	if !ok {
		m.refreshViewportContent()
		return
	}
	if item.path == m.currentFile {
		return
	}

	data, err := os.ReadFile(item.path)
	if err != nil {
		m.viewport.SetContent("Ошибка чтения файла: " + err.Error())
		return
	}

	m.currentFile = item.path
	m.rawMD = string(data)
	m.viewport.GotoTop()

	if m.mode == modeHelp {
		m.mode = modeView
	}
	m.refreshViewportContent()
}

func (m *model) saveCurrentFile() error {
	if m.currentFile == "" {
		return fmt.Errorf("файл не выбран")
	}
	newContent := m.textarea.Value()
	if err := os.WriteFile(m.currentFile, []byte(newContent), 0644); err != nil {
		return err
	}
	m.rawMD = newContent
	delete(m.renderCache, m.currentFile)

	if items, err := loadMarkdownFiles(m.targetDir); err == nil {
		m.list.SetItems(items)
	}
	return nil
}

func (m *model) createNewFile(rawInput string) error {
	cleanInput := strings.TrimSpace(rawInput)
	if cleanInput == "" || strings.HasSuffix(filepath.ToSlash(cleanInput), "/") {
		return fmt.Errorf("укажите имя файла после названия каталога")
	}

	relPath := filepath.FromSlash(cleanInput)
	ext := strings.ToLower(filepath.Ext(relPath))
	if ext != ".md" && ext != ".markdown" {
		relPath += ".md"
	}

	fullPath := filepath.Clean(filepath.Join(m.targetDir, relPath))

	relCheck, err := filepath.Rel(m.targetDir, fullPath)
	if err != nil || strings.HasPrefix(relCheck, "..") {
		return fmt.Errorf("недопустимый путь за пределами рабочей папки")
	}

	targetFolder := filepath.Dir(fullPath)
	if err := os.MkdirAll(targetFolder, 0755); err != nil {
		return fmt.Errorf("не удалось создать каталог: %v", err)
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		title := strings.TrimSuffix(filepath.Base(fullPath), filepath.Ext(fullPath))
		initialContent := fmt.Sprintf("# %s\n\n", title)
		if err := os.WriteFile(fullPath, []byte(initialContent), 0644); err != nil {
			return err
		}
	}

	items, err := loadMarkdownFiles(m.targetDir)
	if err != nil {
		return err
	}
	m.list.SetItems(items)
	m.allDirs = loadDirectories(m.targetDir)

	for idx, it := range items {
		if fi, ok := it.(fileItem); ok && fi.path == fullPath {
			m.list.Select(idx)
			break
		}
	}

	m.currentFile = ""
	m.loadSelectedFile()
	return nil
}

func (m *model) openNewFileDialog() {
	m.mode = modeNewFile
	m.focus = focusReader
	m.allDirs = loadDirectories(m.targetDir)
	m.dirCursor = 0

	defaultPrefix := ""
	if item, ok := m.list.SelectedItem().(fileItem); ok {
		if rel, err := filepath.Rel(m.targetDir, filepath.Dir(item.path)); err == nil && rel != "." {
			defaultPrefix = filepath.ToSlash(rel) + "/"
		}
	}

	m.nameInput.SetValue(defaultPrefix)
	m.nameInput.CursorEnd()
	m.nameInput.Focus()
	m.updateDirSuggestions()
	m.statusMsg = ""
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.mode == modeNewFile {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.mode = modeView
				m.nameInput.Blur()
				m.statusMsg = "Создание отменено"
				return m, nil
			case "up":
				if m.dirCursor > 0 {
					m.dirCursor--
				}
				return m, nil
			case "down":
				if m.dirCursor < len(m.filteredDir)-1 {
					m.dirCursor++
				}
				return m, nil
			case "tab":
				m.applySelectedDir()
				return m, nil
			case "enter":
				if err := m.createNewFile(m.nameInput.Value()); err != nil {
					m.statusMsg = "Ошибка: " + err.Error()
					return m, nil
				}
				m.nameInput.Blur()
				relCreated, _ := filepath.Rel(m.targetDir, m.currentFile)
				m.enterEditMode("Создан: " + filepath.ToSlash(relCreated))
				return m, nil
			}

			m.nameInput, cmd = m.nameInput.Update(msg)
			m.updateDirSuggestions()
			return m, cmd
		}

		if m.mode == modeEdit {
			if m.popupActive {
				switch msg.String() {
				case "up":
					if m.popupIndex > 0 {
						m.popupIndex--
					}
					return m, nil
				case "down":
					if m.popupIndex < len(m.popupMatches)-1 {
						m.popupIndex++
					}
					return m, nil
				case "enter", "tab":
					m.applySelectedSnippet()
					return m, nil
				case "esc", " ":
					m.popupActive = false
					m.popupFilter = ""
					if msg.String() == "esc" {
						return m, nil
					}
				case "backspace":
					if len(m.popupFilter) <= 1 {
						m.popupActive = false
						m.popupFilter = ""
					} else {
						runes := []rune(m.popupFilter)
						m.popupFilter = string(runes[:len(runes)-1])
						m.updatePopupMatches()
					}
				default:
					if len(msg.Runes) == 1 {
						m.popupFilter += string(msg.Runes)
						m.updatePopupMatches()
					}
				}
			}

			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit

			case "esc":
				m.exitEditMode(modeView, "Редактирование отменено")
				return m, nil

			case "ctrl+s":
				if err := m.saveCurrentFile(); err != nil {
					m.statusMsg = "Ошибка сохранения: " + err.Error()
				} else {
					m.exitEditMode(modeView, "Сохранено: "+filepath.Base(m.currentFile))
				}
				return m, nil

			// Только F2 отвечает за переключение панели предпросмотра
			case "f2":
				m.splitPreview = !m.splitPreview
				m.updateLayout()
				return m, nil

			// Ctrl+B снова отвечает за вставку жирного текста
			case "ctrl+b":
				m.textarea.InsertString("**жирный**")
				if m.splitPreview {
					m.refreshViewportContent()
				}
				return m, nil

			case "ctrl+k":
				m.textarea.InsertString("[текст](https://)")
				if m.splitPreview {
					m.refreshViewportContent()
				}
				return m, nil

			case "ctrl+t":
				m.textarea.InsertString("\n| Заголовок 1 | Заголовок 2 |\n| :--- | :--- |\n| Ячейка 1 | Ячейка 2 |\n")
				if m.splitPreview {
					m.refreshViewportContent()
				}
				return m, nil

			case "f1":
				m.exitEditMode(modeHelp, "")
				m.viewport.GotoTop()
				return m, nil

			case "/":
				if !m.popupActive {
					m.popupActive = true
					m.popupFilter = "/"
					m.popupIndex = 0
					m.updatePopupMatches()
				}
			}

			m.textarea, cmd = m.textarea.Update(msg)
			if m.splitPreview {
				m.refreshViewportContent()
			}
			return m, cmd
		}

		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "esc":
			if m.mode == modeHelp {
				m.mode = modeView
				m.viewport.GotoTop()
				m.refreshViewportContent()
				return m, nil
			}

		case "n":
			m.openNewFileDialog()
			return m, nil

		case "?", "h":
			if m.mode == modeHelp {
				m.mode = modeView
			} else {
				m.mode = modeHelp
				m.focus = focusReader
			}
			m.viewport.GotoTop()
			m.refreshViewportContent()
			return m, nil

		case "e":
			if m.currentFile != "" {
				m.enterEditMode("Режим редактирования")
				return m, nil
			}

		case "tab":
			if m.showSidebar {
				if m.focus == focusList {
					m.focus = focusReader
				} else {
					m.focus = focusList
				}
			}
			return m, nil

		case "b":
			m.showSidebar = !m.showSidebar
			if !m.showSidebar {
				m.focus = focusReader
			}
			m.updateLayout()
			return m, nil

		case "enter":
			if m.focus == focusList {
				m.loadSelectedFile()
				m.focus = focusReader
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			m.viewport = viewport.New(10, 10)
			m.ready = true
			m.updateLayout()
			m.loadSelectedFile()
		} else {
			m.updateLayout()
		}
	}

	if m.focus == focusList && m.showSidebar {
		prevIndex := m.list.Index()
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
		if m.list.Index() != prevIndex || m.currentFile == "" {
			m.loadSelectedFile()
		}
	} else if m.mode == modeView || m.mode == modeHelp {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) renderPopupView() string {
	var rows []string
	rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true).Render("Вставка разметки (↑/↓, Enter, Esc):"))

	maxItems := 6
	for idx, item := range m.popupMatches {
		if idx >= maxItems {
			break
		}
		line := fmt.Sprintf("%-8s — %s", item.trigger, item.label)
		if idx == m.popupIndex {
			rows = append(rows, popupSelected.Render(" > "+line+" "))
		} else {
			rows = append(rows, popupNormal.Render("   "+line+" "))
		}
	}
	return popupStyle.Render(strings.Join(rows, "\n"))
}

func (m model) renderNewFileDialog() string {
	inputVal := filepath.ToSlash(strings.TrimSpace(m.nameInput.Value()))
	dirPart := "."
	if idx := strings.LastIndex(inputVal, "/"); idx != -1 {
		if inputVal[:idx] != "" {
			dirPart = inputVal[:idx]
		}
	}

	fullTargetDir := filepath.Join(m.targetDir, filepath.FromSlash(dirPart))
	dirStatus := msgStyle.Render("[Существующий каталог]")
	if info, err := os.Stat(fullTargetDir); os.IsNotExist(err) || (err == nil && !info.IsDir()) {
		dirStatus = warnStyle.Render("[Будет создан новый каталог: " + dirPart + "]")
	}

	var dirRows []string
	maxDirs := 8
	for idx, d := range m.filteredDir {
		if idx >= maxDirs {
			dirRows = append(dirRows, statusStyle.Render(fmt.Sprintf("   ... еще %d каталогов", len(m.filteredDir)-maxDirs)))
			break
		}
		displayDir := d + "/"
		if d == "." {
			displayDir = "./ (Корневой каталог)"
		}
		if idx == m.dirCursor {
			dirRows = append(dirRows, popupSelected.Render(" > "+displayDir+" "))
		} else {
			dirRows = append(dirRows, popupNormal.Render("   "+displayDir+" "))
		}
	}
	if len(dirRows) == 0 {
		dirRows = append(dirRows, statusStyle.Render("   Нет совпадений среди существующих папок — будет создана новая цепочка каталогов"))
	}

	errBlock := ""
	if strings.HasPrefix(m.statusMsg, "Ошибка:") {
		errBlock = "\n" + warnStyle.Render(m.statusMsg) + "\n"
	}

	content := fmt.Sprintf(
		"Создание нового Markdown-документа\n\n"+
			"Путь и имя файла (например: notes/work/plan.md):\n"+
			"%s\n"+
			"Целевой каталог: %s%s\n\n"+
			"Существующие каталоги (↑/↓ — выбор, Tab — подставить в путь):\n"+
			"%s\n\n"+
			"%s",
		m.nameInput.View(),
		dirStatus,
		errBlock,
		strings.Join(dirRows, "\n"),
		statusStyle.Render("Enter: создать файл (и папки) • Tab: вставить выбранную папку • Esc: отмена"),
	)

	return lipgloss.NewStyle().Padding(1, 3).Render(content)
}

func (m model) View() string {
	if !m.ready {
		return "\n  Инициализация..."
	}

	var rightPaneRendered string

	if m.mode == modeEdit {
		hintBar := hintBarStyle.Width(m.textarea.Width()).Render(m.getContextHint())
		editorBlock := lipgloss.JoinVertical(lipgloss.Left, hintBar, m.textarea.View())

		if m.popupActive && len(m.popupMatches) > 0 {
			popupBox := m.renderPopupView()
			editorBlock = lipgloss.JoinVertical(lipgloss.Left, hintBar, popupBox, m.textarea.View())
		}

		editBox := editBorder.Width(m.textarea.Width()).Height(m.viewport.Height).Render(editorBlock)

		if m.splitPreview {
			previewBox := activeBorder.Width(m.viewport.Width).Height(m.viewport.Height).Render(m.viewport.View())
			rightPaneRendered = lipgloss.JoinHorizontal(lipgloss.Top, editBox, previewBox)
		} else {
			rightPaneRendered = editBox
		}
	} else {
		var rightContent string
		if m.mode == modeNewFile {
			rightContent = m.renderNewFileDialog()
		} else {
			rightContent = m.viewport.View()
		}

		rStyle := inactiveBorder
		if m.focus == focusReader || !m.showSidebar {
			switch m.mode {
			case modeHelp:
				rStyle = helpBorder
			case modeNewFile:
				rStyle = newFileBorder
			default:
				rStyle = activeBorder
			}
		}
		rightPaneRendered = rStyle.Width(m.viewport.Width).Height(m.viewport.Height).Render(rightContent)
	}

	var mainView string
	if m.showSidebar {
		lStyle := inactiveBorder
		if m.focus == focusList {
			lStyle = activeBorder
		}
		leftPane := lStyle.Width(sidebarWidth - 2).Height(m.viewport.Height).Render(m.list.View())
		mainView = lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPaneRendered)
	} else {
		mainView = rightPaneRendered
	}

	var footerText string
	switch m.mode {
	case modeNewFile:
		footerText = "НОВЫЙ ФАЙЛ • ↑/↓ + Tab: выбрать папку • Enter: создать • Esc: отмена"
	case modeEdit:
		previewState := "Выкл"
		if m.splitPreview {
			previewState = "Вкл"
		}
		footerText = fmt.Sprintf(
			"РЕДАКТОР [%s] • F2: превью (%s) • /: сниппеты • Ctrl+B: жирный • Ctrl+S: сохр • Esc: выход",
			filepath.Base(m.currentFile), previewState,
		)
	case modeHelp:
		footerText = "СПРАВКА • ↑/↓: прокрутка • ? / Esc: вернуться • e: редактор"
	default:
		focusName := "Список"
		if m.focus == focusReader {
			focusName = "Просмотр"
		}
		statusExtra := ""
		if m.statusMsg != "" {
			statusExtra = " • " + msgStyle.Render(m.statusMsg)
		}
		footerText = fmt.Sprintf(
			"[%s] • n: новый • e: править • ?: справка • Tab: панель • b: сайдбар • %3.f%% • q: выход%s",
			focusName, m.viewport.ScrollPercent()*100, statusExtra,
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left, mainView, statusStyle.Render(footerText))
}

func loadDirectories(root string) []string {
	dirs := []string{"."}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		name := d.Name()
		if (strings.HasPrefix(name, ".") && name != ".") || name == "node_modules" || name == "vendor" {
			return filepath.SkipDir
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr == nil && rel != "." {
			dirs = append(dirs, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(dirs)
	return dirs
}

func loadMarkdownFiles(root string) ([]list.Item, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		root = filepath.Dir(root)
	}

	var items []list.Item

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			if (strings.HasPrefix(name, ".") && name != ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext == ".md" || ext == ".markdown" {
			fileInfo, _ := d.Info()
			sizeKB := float64(0)
			if fileInfo != nil {
				sizeKB = float64(fileInfo.Size()) / 1024.0
			}

			relName, relErr := filepath.Rel(root, path)
			if relErr != nil {
				relName = d.Name()
			}

			items = append(items, fileItem{
				name: filepath.ToSlash(relName),
				path: path,
				info: fmt.Sprintf("%.1f KB", sizeKB),
			})
		}
		return nil
	})

	return items, err
}

func main() {
	targetPath, err := os.Getwd()
	if err != nil {
		targetPath = "."
	}

	var initialFile string

	if len(os.Args) > 1 {
		if absPath, err := filepath.Abs(os.Args[1]); err == nil {
			targetPath = absPath
		} else {
			targetPath = os.Args[1]
		}
	}

	targetDir := targetPath
	if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
		initialFile = targetPath
		targetDir = filepath.Dir(targetPath)
	}

	styleName := "dark"
	if !lipgloss.HasDarkBackground() {
		styleName = "light"
	}

	items, loadErr := loadMarkdownFiles(targetDir)

	delegate := list.NewDefaultDelegate()
	fileList := list.New(items, delegate, 0, 0)
	fileList.Title = "Документы"
	fileList.SetShowHelp(false)

	if initialFile != "" {
		for idx, it := range items {
			if fi, ok := it.(fileItem); ok && fi.path == initialFile {
				fileList.Select(idx)
				break
			}
		}
	}

	ta := textarea.New()
	ta.ShowLineNumbers = true
	ta.CharLimit = 0
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	ta.FocusedStyle.LineNumber = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	ta.FocusedStyle.CursorLineNumber = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)

	ti := textinput.New()
	ti.Placeholder = "folder/subfolder/note.md"
	ti.CharLimit = 256

	statusMsg := ""
	if loadErr != nil {
		statusMsg = "Ошибка чтения каталога: " + loadErr.Error()
	}

	m := model{
		list:              fileList,
		textarea:          ta,
		nameInput:         ti,
		renderCache:       make(map[string]string),
		styleName:         styleName,
		targetDir:         targetDir,
		allDirs:           loadDirectories(targetDir),
		statusMsg:         statusMsg,
		focus:             focusList,
		mode:              modeView,
		showSidebar:       true,
		sidebarBeforeEdit: true,
		splitPreview:      false,
	}

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Ошибка: %v\n", err)
		os.Exit(1)
	}
}
