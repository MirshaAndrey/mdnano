package main

import (
	"fmt"
	"os"
	"path/filepath"
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

const sidebarWidth = 32

const markdownCheatsheet = `# Шпаргалка по Markdown

## 1. Заголовки
` + "```markdown" + `
# Заголовок 1 уровня (H1)
## Заголовок 2 уровня (H2)
### Заголовок 3 уровня (H3)
` + "```" + `

## 2. Форматирование текста
* **Жирный текст**: ` + "`**жирный**` или `__жирный__`" + `
* *Курсив*: ` + "`*курсив*` или `_курсив_`" + `
* ~~Зачеркнутый~~: ` + "`~~зачеркнутый~~`" + `
* ` + "`Инлайн-код`" + `: обратные кавычки ` + "`` `код` ``" + `

## 3. Списки и задачи
` + "```markdown" + `
- Элемент маркированного списка
  - Вложенный элемент (отступ 2 пробела)
1. Первый элемент нумерованного списка
2. Второй элемент

- [x] Выполненная задача
- [ ] Открытая задача
` + "```" + `

## 4. Ссылки, цитаты и разделители
` + "```markdown" + `
[Текст ссылки](https://example.com)
![Описание картинки](path/to/image.png)

> Это блочная цитата.
> Может занимать несколько строк.

--- (Горизонтальная линия)
` + "```" + `

## 5. Блоки кода и таблицы
Для создания блока кода используйте три обратные кавычки (` + "```" + `) и укажите язык (например, ` + "`go`, `bash`, `json`" + `).

| Заголовок 1 | Выравнивание по центру | По правому краю |
| :--- | :---: | ---: |
| Ячейка 1 | Ячейка 2 | 100 $ |
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

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			PaddingLeft(1)

	msgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
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
	list          list.Model
	viewport      viewport.Model
	textarea      textarea.Model
	nameInput     textinput.Model
	renderer      *glamour.TermRenderer
	renderCache   map[string]string
	styleName     string
	lastWrapWidth int
	targetDir     string
	focus         focusState
	mode          rightPaneMode
	showSidebar   bool
	rawMD         string
	currentFile   string
	statusMsg     string
	width         int
	height        int
	ready         bool
}

func (m model) Init() tea.Cmd {
	return nil
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

	vpWidth := m.width - 2
	if m.showSidebar {
		vpWidth = m.width - sidebarWidth - 2
	}
	if vpWidth < 20 {
		vpWidth = 20
	}

	m.viewport.Width = vpWidth
	m.viewport.Height = contentHeight

	m.textarea.SetWidth(vpWidth)
	m.textarea.SetHeight(contentHeight)
	m.nameInput.Width = vpWidth - 10

	m.ensureRenderer(vpWidth)
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

	items := loadMarkdownFiles(m.targetDir)
	m.list.SetItems(items)
	return nil
}

func (m *model) createNewFile(rawName string) error {
	name := strings.TrimSpace(rawName)
	if name == "" {
		return fmt.Errorf("имя файла не может быть пустым")
	}

	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".md" && ext != ".markdown" {
		name += ".md"
	}

	fullPath := filepath.Join(m.targetDir, name)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		title := strings.TrimSuffix(name, filepath.Ext(name))
		initialContent := fmt.Sprintf("# %s\n\n", title)
		if err := os.WriteFile(fullPath, []byte(initialContent), 0644); err != nil {
			return err
		}
	}

	items := loadMarkdownFiles(m.targetDir)
	m.list.SetItems(items)

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

			case "enter":
				if err := m.createNewFile(m.nameInput.Value()); err != nil {
					m.statusMsg = "Ошибка: " + err.Error()
					return m, nil
				}
				m.nameInput.Blur()
				m.mode = modeEdit
				m.focus = focusReader
				m.textarea.SetValue(m.rawMD)
				m.textarea.Focus()
				m.statusMsg = "Создан: " + filepath.Base(m.currentFile)
				return m, nil
			}

			m.nameInput, cmd = m.nameInput.Update(msg)
			return m, cmd
		}

		if m.mode == modeEdit {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit

			case "esc":
				m.mode = modeView
				m.textarea.Blur()
				m.statusMsg = "Редактирование отменено"
				return m, nil

			case "ctrl+s":
				if err := m.saveCurrentFile(); err != nil {
					m.statusMsg = "Ошибка сохранения: " + err.Error()
				} else {
					m.statusMsg = "Сохранено: " + filepath.Base(m.currentFile)
					m.mode = modeView
					m.textarea.Blur()
					m.refreshViewportContent()
				}
				return m, nil

			case "f1":
				m.mode = modeHelp
				m.textarea.Blur()
				m.viewport.GotoTop()
				m.refreshViewportContent()
				return m, nil
			}

			m.textarea, cmd = m.textarea.Update(msg)
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
			m.mode = modeNewFile
			m.focus = focusReader
			m.nameInput.SetValue("")
			m.nameInput.Focus()
			m.statusMsg = ""
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
				m.mode = modeEdit
				m.focus = focusReader
				m.textarea.SetValue(m.rawMD)
				m.textarea.Focus()
				m.statusMsg = "Режим редактирования"
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

func (m model) View() string {
	if !m.ready {
		return "\n  Инициализация..."
	}

	var rightContent string
	switch m.mode {
	case modeEdit:
		rightContent = m.textarea.View()
	case modeNewFile:
		dialog := lipgloss.NewStyle().Padding(2, 4).Render(
			"Создание нового Markdown-документа\n\n" +
				"Введите имя файла (расширение .md добавится автоматически):\n\n" +
				m.nameInput.View() + "\n\n" +
				statusStyle.Render("Enter: создать и открыть в редакторе • Esc: отмена"),
		)
		rightContent = dialog
	default:
		rightContent = m.viewport.View()
	}

	rStyle := inactiveBorder
	if m.focus == focusReader || !m.showSidebar {
		switch m.mode {
		case modeEdit:
			rStyle = editBorder
		case modeHelp:
			rStyle = helpBorder
		case modeNewFile:
			rStyle = newFileBorder
		default:
			rStyle = activeBorder
		}
	}

	var mainView string
	if m.showSidebar {
		lStyle := inactiveBorder
		if m.focus == focusList {
			lStyle = activeBorder
		}
		leftPane := lStyle.Width(sidebarWidth - 2).Height(m.viewport.Height).Render(m.list.View())
		rightPane := rStyle.Width(m.viewport.Width).Height(m.viewport.Height).Render(rightContent)
		mainView = lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	} else {
		mainView = rStyle.Width(m.viewport.Width).Height(m.viewport.Height).Render(rightContent)
	}

	var footerText string
	switch m.mode {
	case modeNewFile:
		footerText = "НОВЫЙ ФАЙЛ • Enter: подтвердить • Esc: отмена"
	case modeEdit:
		footerText = fmt.Sprintf(
			"РЕДАКТОР [%s] • Ctrl+S: сохранить • Esc: отмена • F1: шпаргалка",
			filepath.Base(m.currentFile),
		)
	case modeHelp:
		footerText = "СПРАВКА ПО MARKDOWN • ↑/↓: прокрутка • ? / Esc: вернуться • e: редактор"
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

func loadMarkdownFiles(dir string) []list.Item {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var items []list.Item
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".md" || ext == ".markdown" {
			info, _ := entry.Info()
			sizeKB := float64(0)
			if info != nil {
				sizeKB = float64(info.Size()) / 1024.0
			}
			items = append(items, fileItem{
				name: entry.Name(),
				path: filepath.Join(dir, entry.Name()),
				info: fmt.Sprintf("%.1f KB", sizeKB),
			})
		}
	}
	return items
}

func main() {
	targetDir := "."
	if len(os.Args) > 1 {
		targetDir = os.Args[1]
	}

	styleName := "dark"
	if !lipgloss.HasDarkBackground() {
		styleName = "light"
	}

	items := loadMarkdownFiles(targetDir)

	delegate := list.NewDefaultDelegate()
	fileList := list.New(items, delegate, 0, 0)
	fileList.Title = "Документы"
	fileList.SetShowHelp(false)

	ta := textarea.New()
	ta.ShowLineNumbers = true
	ta.CharLimit = 0

	ti := textinput.New()
	ti.Placeholder = "example.md"
	ti.CharLimit = 128

	m := model{
		list:        fileList,
		textarea:    ta,
		nameInput:   ti,
		renderCache: make(map[string]string),
		styleName:   styleName,
		targetDir:   targetDir,
		focus:       focusList,
		mode:        modeView,
		showSidebar: true,
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
