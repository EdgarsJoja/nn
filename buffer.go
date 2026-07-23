package main

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Point struct{ X, Y int }

type Selection struct {
	Active bool
	Start  Point
	End    Point
}

type undoState struct {
	buffer    []string
	cursor    Point
	selection Selection
	modified  bool
}

type FileTab struct {
	filename     string
	filepath     string
	buffer       []string
	cursor       Point
	offset       Point
	selection    Selection
	modified     bool
	syntaxTokens []syntaxToken
	gitLineStat  []byte
	headContent  string
}

func (e *Editor) activeFile() *FileTab {
	return e.openFiles[e.activeTab]
}

const maxUndo = 100

const (
	opNone     = 0
	opInsert   = 1
	opDeleteBk = 2
	opDeleteFd = 3
)

func (e *Editor) saveUndoState(op int) {
	if op == e.lastOp && len(e.undoStack) > 0 {
		return
	}
	state := undoState{
		buffer:    append([]string{}, e.buffer...),
		cursor:    e.cursor,
		selection: e.selection,
		modified:  e.modified,
	}
	e.undoStack = append(e.undoStack, state)
	if len(e.undoStack) > maxUndo {
		e.undoStack = e.undoStack[1:]
	}
	e.redoStack = nil
	e.lastOp = op
}

func (e *Editor) undo() {
	if len(e.undoStack) == 0 {
		return
	}
	redo := undoState{
		buffer:    append([]string{}, e.buffer...),
		cursor:    e.cursor,
		selection: e.selection,
		modified:  e.modified,
	}
	e.redoStack = append(e.redoStack, redo)

	state := e.undoStack[len(e.undoStack)-1]
	e.undoStack = e.undoStack[:len(e.undoStack)-1]

	e.buffer = state.buffer
	e.cursor = state.cursor
	e.selection = state.selection
	e.modified = state.modified
	e.openFiles[e.activeTab].buffer = e.buffer
	e.openFiles[e.activeTab].cursor = e.cursor
	e.openFiles[e.activeTab].selection = e.selection
	e.openFiles[e.activeTab].modified = e.modified
	e.openFiles[e.activeTab].syntaxTokens = nil
	e.lastOp = opNone
	e.cursorInBounds()
	e.updateGitLineStat()
}

func (e *Editor) redo() {
	if len(e.redoStack) == 0 {
		return
	}
	undo := undoState{
		buffer:    append([]string{}, e.buffer...),
		cursor:    e.cursor,
		selection: e.selection,
		modified:  e.modified,
	}
	e.undoStack = append(e.undoStack, undo)

	state := e.redoStack[len(e.redoStack)-1]
	e.redoStack = e.redoStack[:len(e.redoStack)-1]

	e.buffer = state.buffer
	e.cursor = state.cursor
	e.selection = state.selection
	e.modified = state.modified
	e.openFiles[e.activeTab].buffer = e.buffer
	e.openFiles[e.activeTab].cursor = e.cursor
	e.openFiles[e.activeTab].selection = e.selection
	e.openFiles[e.activeTab].modified = e.modified
	e.openFiles[e.activeTab].syntaxTokens = nil
	e.lastOp = opNone
	e.cursorInBounds()
	e.updateGitLineStat()
}

func (e *Editor) setModified() {
	e.modified = true
	if e.activeTab < len(e.openFiles) {
		e.openFiles[e.activeTab].modified = true
		e.openFiles[e.activeTab].syntaxTokens = nil
	}
	e.gitDirty = true
	e.tokenizeDebounce = 5
}

func (e *Editor) saveCurrentTab() {
	if e.activeTab < len(e.openFiles) {
		e.openFiles[e.activeTab].buffer = e.buffer
		e.openFiles[e.activeTab].cursor = e.cursor
		e.openFiles[e.activeTab].offset = e.offset
		e.openFiles[e.activeTab].selection = e.selection
		e.openFiles[e.activeTab].filename = e.filename
		e.openFiles[e.activeTab].modified = e.modified
		e.openFiles[e.activeTab].syntaxTokens = nil
	}
}

func (e *Editor) restoreTab(idx int) {
	if idx < 0 || idx >= len(e.openFiles) {
		return
	}
	t := e.openFiles[idx]
	e.buffer = t.buffer
	e.cursor = t.cursor
	e.offset = t.offset
	e.selection = t.selection
	e.filename = t.filename
	e.modified = t.modified
	e.activeTab = idx
	e.adjustTabOffset()
}

func (e *Editor) tabWidth(t *FileTab) int {
	label := filepath.Base(t.filename)
	if t.filename == "" {
		label = "untitled"
	}
	if t.modified {
		label += " •"
	}
	return len([]rune(" " + label + " ")) + 1
}

func (e *Editor) adjustTabOffset() {
	if len(e.openFiles) == 0 {
		e.tabOffset = 0
		return
	}

	if e.activeTab < e.tabOffset {
		e.tabOffset = e.activeTab
	}

	_, _, w, _ := e.editorBox.GetRect()
	availW := w

	x := 0
	targetEnd := 0
	for i := e.tabOffset; i < len(e.openFiles); i++ {
		w := e.tabWidth(e.openFiles[i])
		pos := x + w
		if i == e.activeTab {
			targetEnd = pos
		}
		x = pos
	}

	if availW < 40 {
		availW = 40
	}

	for targetEnd > availW && e.tabOffset < e.activeTab {
		targetEnd -= e.tabWidth(e.openFiles[e.tabOffset])
		e.tabOffset++
	}
}

func (e *Editor) switchTab(dir int) {
	if len(e.openFiles) < 2 {
		return
	}
	e.saveCurrentTab()
	next := (e.activeTab + dir) % len(e.openFiles)
	if next < 0 {
		next = len(e.openFiles) - 1
	}
	e.restoreTab(next)
	e.msg("tab: " + filepath.Base(e.openFiles[next].filename))
	e.refreshGitTab()
}

func (e *Editor) closeTab() {
	if len(e.openFiles) <= 1 {
		e.buffer = []string{""}
		e.cursor = Point{}
		e.offset = Point{}
		e.filename = ""
		e.modified = false
		e.selection = Selection{}
		e.openFiles[0].buffer = []string{""}
		e.openFiles[0].filename = ""
		e.openFiles[0].modified = false
		e.msg("closed tab")
		e.mode = "sidebar"
		e.app.SetFocus(e.sidebar)
		return
	}
	e.saveCurrentTab()
	idx := e.activeTab
	e.openFiles = append(e.openFiles[:idx], e.openFiles[idx+1:]...)
	if e.activeTab >= len(e.openFiles) {
		e.activeTab = len(e.openFiles) - 1
	}
	e.restoreTab(e.activeTab)
	e.msg("closed tab")
}

func (e *Editor) loadFile(path string) {
	for i, tab := range e.openFiles {
		if tab.filename == path {
			e.saveCurrentTab()
			e.restoreTab(i)
			e.msg("switched to " + filepath.Base(path))
			e.refreshGit()
			return
		}
	}

	if len(e.openFiles) == 1 && e.openFiles[0].filename == "" && len(e.openFiles[0].buffer) == 1 && e.openFiles[0].buffer[0] == "" {
		tab := e.openFiles[0]
		data, err := os.ReadFile(path)
		if err != nil {
			e.msg("error: " + err.Error())
			return
		}
		tab.buffer = strings.Split(string(data), "\n")
		if len(tab.buffer) == 0 {
			tab.buffer = []string{""}
		}
		tab.filename = path
		tab.syntaxTokens = nil
		e.undoStack = nil
		e.redoStack = nil
		e.restoreTab(0)
		e.msg("opened " + filepath.Base(path))
		e.refreshGit()
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		e.msg("error: " + err.Error())
		return
	}
	e.saveCurrentTab()
	content := string(data)
	buf := strings.Split(content, "\n")
	if len(buf) == 0 {
		buf = []string{""}
	}
	e.openFiles = append(e.openFiles, &FileTab{
		filename:     path,
		buffer:       buf,
		syntaxTokens: nil,
	})
	e.undoStack = nil
	e.redoStack = nil
	e.restoreTab(len(e.openFiles) - 1)
	e.msg("opened " + filepath.Base(path))
	e.refreshGit()
}

func (e *Editor) saveFile(path string) {
	data := []byte(strings.Join(e.buffer, "\n"))
	if err := os.WriteFile(path, data, 0644); err != nil {
		e.msg("error: " + err.Error())
		return
	}
	e.filename = path
	e.modified = false
	e.saveCurrentTab()
	e.msg("saved " + filepath.Base(path))
}

func (e *Editor) selectedText() string {
	if !e.selection.Active {
		return ""
	}
	start, end := e.selection.Start, e.selection.End
	if start.Y > end.Y || (start.Y == end.Y && start.X > end.X) {
		start, end = end, start
	}
	var lines []string
	for y := start.Y; y <= end.Y && y < len(e.buffer); y++ {
		switch {
		case y == start.Y && y == end.Y:
			lines = append(lines, e.buffer[y][start.X:end.X])
		case y == start.Y:
			lines = append(lines, e.buffer[y][start.X:])
		case y == end.Y:
			lines = append(lines, e.buffer[y][:end.X])
		default:
			lines = append(lines, e.buffer[y])
		}
	}
	return strings.Join(lines, "\n")
}

func (e *Editor) deleteSelection() {
	if !e.selection.Active {
		return
	}
	start, end := e.selection.Start, e.selection.End
	if start.Y > end.Y || (start.Y == end.Y && start.X > end.X) {
		start, end = end, start
	}
	if start.Y == end.Y {
		line := e.buffer[start.Y]
		e.buffer[start.Y] = line[:start.X] + line[end.X:]
	} else {
		first := e.buffer[start.Y][:start.X]
		last := e.buffer[end.Y][end.X:]
		e.buffer[start.Y] = first + last
		e.buffer = append(e.buffer[:start.Y+1], e.buffer[end.Y+1:]...)
	}
	e.cursor = start
	e.selection = Selection{}
	e.setModified()
}

func (e *Editor) insertText(text string) {
	e.saveUndoState(opInsert)
	e.deleteSelection()
	for _, ch := range text {
		if ch == '\n' {
			line := e.buffer[e.cursor.Y]
			rest := line[e.cursor.X:]
			e.buffer[e.cursor.Y] = line[:e.cursor.X]
			tail := make([]string, len(e.buffer[e.cursor.Y+1:]))
			copy(tail, e.buffer[e.cursor.Y+1:])
			e.buffer = append(e.buffer[:e.cursor.Y+1], "")
			e.buffer = append(e.buffer, tail...)
			e.buffer[e.cursor.Y+1] = rest
			e.cursor.Y++
			e.cursor.X = 0
		} else {
			line := e.buffer[e.cursor.Y]
			e.buffer[e.cursor.Y] = line[:e.cursor.X] + string(ch) + line[e.cursor.X:]
			e.cursor.X++
		}
	}
	e.setModified()
}

func (e *Editor) copySel() {
	if t := e.selectedText(); t != "" {
		e.clipboard = t
		e.msg("copied")
	}
}

func (e *Editor) cutSel() {
	e.saveUndoState(opNone)
	if t := e.selectedText(); t != "" {
		e.clipboard = t
		e.deleteSelection()
		e.msg("cut")
	}
}

func (e *Editor) pasteClip() {
	e.saveUndoState(opNone)
	e.deleteSelection()
	if e.clipboard == "" {
		return
	}
	lines := strings.Split(e.clipboard, "\n")
	if len(lines) == 1 {
		line := e.buffer[e.cursor.Y]
		e.buffer[e.cursor.Y] = line[:e.cursor.X] + lines[0] + line[e.cursor.X:]
		e.cursor.X += len([]rune(lines[0]))
	} else {
		line := e.buffer[e.cursor.Y]
		rest := line[e.cursor.X:]
		e.buffer[e.cursor.Y] = line[:e.cursor.X]
		e.buffer[e.cursor.Y] += lines[0]
		tail := make([]string, len(e.buffer[e.cursor.Y+1:]))
		copy(tail, e.buffer[e.cursor.Y+1:])
		e.buffer = append(e.buffer[:e.cursor.Y+1], "", "")
		e.buffer = append(e.buffer, tail...)
		for i := 1; i < len(lines)-1; i++ {
			e.buffer = append(e.buffer[:e.cursor.Y+1+i], "", "")
			e.buffer = append(e.buffer, e.buffer[e.cursor.Y+1+i+1:]...)
			e.buffer[e.cursor.Y+1+i] = lines[i]
		}
		e.buffer[e.cursor.Y+len(lines)-1] = lines[len(lines)-1] + rest
		e.cursor.Y += len(lines) - 1
		e.cursor.X = len([]rune(lines[len(lines)-1]))
	}
	e.setModified()
}

func commentPrefix(lang string) string {
	switch lang {
	case "go", "c", "cpp", "java", "javascript", "typescript", "jsx",
		"rust", "swift", "kotlin", "scala", "svelte", "vue", "zig", "ocaml",
		"dart", "php", "protobuf":
		return "// "
	case "python", "ruby", "bash", "r", "yaml", "toml", "makefile",
		"nix", "fish", "terraform", "dockerfile", "gitignore", "ini",
		"elixir":
		return "# "
	case "sql", "lua":
		return "-- "
	}
	return "// "
}

func unindentLine(line *string) bool {
	if len(*line) > 0 && (*line)[0] == '\t' {
		*line = (*line)[1:]
		return true
	}
	spaces := 0
	for spaces < 4 && spaces < len(*line) && (*line)[spaces] == ' ' {
		spaces++
	}
	if spaces > 0 {
		*line = (*line)[spaces:]
		return true
	}
	return false
}

func addComment(line, prefix string) string {
	trimmed := strings.TrimLeft(line, " \t")
	leadingWS := line[:len(line)-len(trimmed)]
	return leadingWS + prefix + trimmed
}

func uncommentLine(line, prefix string) string {
	trimmed := strings.TrimLeft(line, " \t")
	leadingWS := line[:len(line)-len(trimmed)]
	if strings.HasPrefix(trimmed, prefix) {
		return leadingWS + trimmed[len(prefix):]
	}
	return line
}

func (e *Editor) indentSelection() {
	e.saveUndoState(opNone)
	start, end := e.selection.Start, e.selection.End
	if start.Y > end.Y || (start.Y == end.Y && start.X > end.X) {
		start, end = end, start
	}
	endY := end.Y
	if end.X == 0 && endY > start.Y {
		endY--
	}
	for y := start.Y; y <= endY; y++ {
		e.buffer[y] = "\t" + e.buffer[y]
	}
	if e.cursor.Y >= start.Y && e.cursor.Y <= endY {
		e.cursor.X++
	}
	e.selection.Start.Y = start.Y
	e.selection.Start.X = 0
	e.selection.End.Y = endY
	e.selection.End.X = len([]rune(e.buffer[endY]))
	e.selection.Active = true
	e.openFiles[e.activeTab].syntaxTokens = nil
	e.setModified()
}

func (e *Editor) unindentSelection() {
	e.saveUndoState(opNone)

	var startY, endY int
	if e.selection.Active {
		start, end := e.selection.Start, e.selection.End
		if start.Y > end.Y || (start.Y == end.Y && start.X > end.X) {
			start, end = end, start
		}
		startY = start.Y
		endY = end.Y
		if end.X == 0 && endY > startY {
			endY--
		}
	} else {
		startY = e.cursor.Y
		endY = e.cursor.Y
	}

	anyUnindented := false
	for y := startY; y <= endY; y++ {
		if unindentLine(&e.buffer[y]) {
			anyUnindented = true
		}
	}
	if !anyUnindented {
		return
	}

	if e.cursor.Y >= startY && e.cursor.Y <= endY {
		e.cursor.X--
		if e.cursor.X < 0 {
			e.cursor.X = 0
		}
	}

	if e.selection.Active {
		e.selection.Start.Y = startY
		e.selection.Start.X = 0
		e.selection.End.Y = endY
		e.selection.End.X = len([]rune(e.buffer[endY]))
		e.selection.Active = true
	}
	e.openFiles[e.activeTab].syntaxTokens = nil
	e.setModified()
}

func (e *Editor) toggleComment() {
	e.saveUndoState(opNone)

	prefix := commentPrefix(langFromExt(e.filename))
	if prefix == "" {
		return
	}

	var startY, endY int
	if e.selection.Active {
		start, end := e.selection.Start, e.selection.End
		if start.Y > end.Y || (start.Y == end.Y && start.X > end.X) {
			start, end = end, start
		}
		startY = start.Y
		endY = end.Y
		if end.X == 0 && endY > startY {
			endY--
		}
	} else {
		startY = e.cursor.Y
		endY = e.cursor.Y
	}

	allCommented := true
	for y := startY; y <= endY; y++ {
		trimmed := strings.TrimLeft(e.buffer[y], " \t")
		if !strings.HasPrefix(trimmed, prefix) {
			allCommented = false
			break
		}
	}

	if allCommented {
		for y := startY; y <= endY; y++ {
			e.buffer[y] = uncommentLine(e.buffer[y], prefix)
		}
		if e.cursor.Y >= startY && e.cursor.Y <= endY {
			e.cursor.X -= len([]rune(prefix))
			if e.cursor.X < 0 {
				e.cursor.X = 0
			}
		}
	} else {
		for y := startY; y <= endY; y++ {
			e.buffer[y] = addComment(e.buffer[y], prefix)
		}
		if e.cursor.Y >= startY && e.cursor.Y <= endY {
			e.cursor.X += len([]rune(prefix))
		}
	}

	if e.selection.Active {
		e.selection.Start.Y = startY
		e.selection.Start.X = 0
		e.selection.End.Y = endY
		e.selection.End.X = len([]rune(e.buffer[endY]))
		e.selection.Active = true
	}
	e.openFiles[e.activeTab].syntaxTokens = nil
	e.setModified()
}

func (e *Editor) scanFiles() {
	e.fuzzyFiles = nil
	root := e.initialDir
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}
		base := d.Name()
		if base == ".git" && d.IsDir() {
			return filepath.SkipDir
		}
		if e.hideDotfiles && strings.HasPrefix(base, ".") && d.IsDir() {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		if e.hideDotfiles && strings.HasPrefix(base, ".") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		e.fuzzyFiles = append(e.fuzzyFiles, fuzzyFileInfo{
			path:  rel,
			lower: []rune(strings.ToLower(rel)),
		})
		return nil
	})
	sort.Slice(e.fuzzyFiles, func(i, j int) bool {
		return e.fuzzyFiles[i].path < e.fuzzyFiles[j].path
	})
}

func (e *Editor) scanTextFiles() []string {
	root := e.initialDir
	var files []string
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}
		base := d.Name()
		if base == ".git" && d.IsDir() {
			return filepath.SkipDir
		}
		if e.hideDotfiles && strings.HasPrefix(base, ".") && d.IsDir() {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		if e.hideDotfiles && strings.HasPrefix(base, ".") {
			return nil
		}
		if isDepDir(path) {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(base))
		if isBinaryExt(ext) {
			return nil
		}
		if isBinaryContent(path) {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		files = append(files, rel)
		return nil
	})
	sort.Strings(files)
	return files
}

func (e *Editor) searchInFiles(query string) {
	e.textSearchQuery = query
	if query == "" {
		e.textSearchResults = nil
		e.textSearchIdx = 0
		e.textSearchOff = 0
		return
	}

	root := e.initialDir
	lower := strings.ToLower(query)

	if e.textSearchCache == nil {
		e.textSearchCache = make(map[string][]string)
		e.textSearchFiles = e.scanTextFiles()
		for _, rel := range e.textSearchFiles {
			fullPath := filepath.Join(root, rel)
			data, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			e.textSearchCache[rel] = strings.Split(string(data), "\n")
		}
	}

	const maxResults = 100
	var results []TextSearchResult

	for _, rel := range e.textSearchFiles {
		if len(results) >= maxResults {
			break
		}
		lines, ok := e.textSearchCache[rel]
		if !ok {
			continue
		}

		for y, line := range lines {
			if len(results) >= maxResults {
				break
			}
			lowerLine := strings.ToLower(line)
			idx := strings.Index(lowerLine, lower)
			if idx == -1 {
				continue
			}

			beforeLines := make([]string, 0, 3)
			for by := y - 3; by < y; by++ {
				if by >= 0 {
					beforeLines = append(beforeLines, lines[by])
				}
			}
			afterLines := make([]string, 0, 3)
			for ay := y + 1; ay <= y+3 && ay < len(lines); ay++ {
				afterLines = append(afterLines, lines[ay])
			}

			results = append(results, TextSearchResult{
				filePath: rel,
				lineNum:  y + 1,
				matchCol: idx,
				line:     line,
				before:   beforeLines,
				after:    afterLines,
			})
		}
	}

	e.textSearchResults = results
	if e.textSearchIdx >= len(results) {
		e.textSearchIdx = 0
	}
	e.textSearchOff = 0
}

var depDirs = []string{
	"/vendor/", "/node_modules/", "/.git/", "/__pycache__/",
	"/.venv/", "/venv/", "/.tox/", "/env/",
	"/target/", "/build/", "/dist/", "/.next/",
	"/bower_components/", "/jspm_packages/", "/.dub/",
}

func isDepDir(path string) bool {
	lower := strings.ToLower(path)
	for _, d := range depDirs {
		if strings.Contains(lower, d) {
			return true
		}
		trimmed := strings.TrimPrefix(d, "/")
		if strings.HasPrefix(lower, trimmed) {
			return true
		}
	}
	return false
}

var binaryExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".ico": true, ".webp": true,
	".mp3": true, ".wav": true, ".ogg": true, ".flac": true, ".aac": true, ".wma": true,
	".mp4": true, ".avi": true, ".mkv": true, ".mov": true, ".wmv": true, ".flv": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true, ".7z": true, ".rar": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true, ".o": true, ".a": true, ".lib": true,
	".db": true, ".sqlite": true, ".sqlite3": true, ".mdb": true,
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".eot": true,
	".pyc": true, ".pyo": true, ".class": true, ".jar": true, ".dex": true,
	".iso": true, ".img": true, ".dmg": true,
	".wasm": true,
	".dat": true, ".pkl": true,
}

func isBinaryExt(ext string) bool {
	return binaryExts[ext]
}

func isBinaryContent(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 8000)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return false
	}
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}

type fuzzyFileInfo struct {
	path  string
	lower []rune
}

type TextSearchResult struct {
	filePath string
	lineNum  int
	matchCol int
	line     string
	before   []string
	after    []string
}

type fuzzyResult struct {
	path    string
	score   int
	matches []int
	fileIdx int
}

func fuzzyScoreRunes(q []rune, t []rune, path string) (int, []int, bool) {
	if len(q) == 0 {
		return 0, nil, true
	}

	bestScore := -1
	var bestMatches []int

	for start := 0; start < len(t); start++ {
		if t[start] != q[0] {
			continue
		}
		score, match := matchFrom(q, t, start)
		if score < 0 {
			continue
		}
		score, match = optimizeMatches(q, t, match)
		if score > bestScore {
			bestScore = score
			bestMatches = match
		}
	}
	if bestScore < 0 {
		return 0, nil, false
	}
	depth := 0
	for _, c := range path {
		if c == '/' {
			depth++
		}
	}
	bestScore -= depth * 10
	if isDepDir(path) {
		bestScore -= 100
	}
	return bestScore, bestMatches, true
}

func optimizeMatches(q, t []rune, m []int) (int, []int) {
	improved := true
	for improved {
		improved = false
		for i := len(m) - 1; i >= 0; i-- {
			qi := i
			cur := m[i]
			nextBound := len(t)
			if i+1 < len(m) {
				nextBound = m[i+1]
			}
			// Look for a later occurrence of q[qi] between cur+1 and nextBound
			bestShift := -1
			for try := cur + 1; try < nextBound; try++ {
				if t[try] == q[qi] {
					bestShift = try
					break
				}
			}
			if bestShift < 0 {
				continue
			}
			oldScore := scoreMatches(q, t, m)
			m[i] = bestShift
			newScore := scoreMatches(q, t, m)
			if newScore > oldScore {
				improved = true
			} else {
				m[i] = cur
			}
		}
	}
	return scoreMatches(q, t, m), m
}

func scoreMatches(q, t []rune, m []int) int {
	score := 0
	prevMatch := -2
	gap := 0
	for _, ti := range m {
		if ti == prevMatch+1 {
			score += 5
		} else {
			score += 1
		}
		if prevMatch >= 0 && ti > prevMatch+1 {
			gap += ti - prevMatch - 1
		}
		if ti > 0 && t[ti-1] == '/' {
			score += 3
		}
		prevMatch = ti
	}
	return score - gap
}

func matchFrom(q, t []rune, start int) (int, []int) {
	qi := 0
	score := 0
	prevMatch := -2
	lastMatch := -1
	gapPenalty := 0
	var m []int
	for ti := start; ti < len(t) && qi < len(q); ti++ {
		if t[ti] == q[qi] {
			if ti == prevMatch+1 {
				score += 5
			} else {
				score += 1
			}
			if lastMatch >= 0 && ti > lastMatch+1 {
				gapPenalty += ti - lastMatch - 1
			}
			if ti > 0 && t[ti-1] == '/' {
				score += 3
			}
			prevMatch = ti
			lastMatch = ti
			m = append(m, ti)
			qi++
		}
	}
	if qi != len(q) {
		return -1, nil
	}
	score -= gapPenalty
	return score, m
}

func (e *Editor) cmdFuzzyFinder() {
	e.restoreStatusBar()
	e.scanFiles()
	e.fuzzyResults = make([]fuzzyResult, len(e.fuzzyFiles))
	for i, f := range e.fuzzyFiles {
		e.fuzzyResults[i] = fuzzyResult{path: f.path, fileIdx: i}
	}
	e.fuzzyIdx = 0
	e.fuzzyOff = 0
	e.fuzzyQuery = ""
	e.showFuzzy = true
	e.mode = "editor"
	e.app.SetFocus(e.editorBox)
	if len(e.fuzzyFiles) == 0 {
		e.msg("fuzzy: no files found")
	} else {
		names := ""
		for i := 0; i < len(e.fuzzyFiles) && i < 3; i++ {
			if i > 0 {
				names += " "
			}
			names += e.fuzzyFiles[i].path
		}
		e.msg("fuzzy: " + strconv.Itoa(len(e.fuzzyFiles)) + " files (" + names + ")")
	}
}

func (e *Editor) fuzzyCancel() {
	e.showFuzzy = false
	e.fuzzyResults = nil
	e.fuzzyFiles = nil
	e.fuzzyQuery = ""
	e.fuzzyPrevQuery = ""
	e.fuzzyPrevCandidates = nil
	e.msg("")
}

func (e *Editor) updateFuzzy(query string) {
	if query == "" {
		e.fuzzyResults = make([]fuzzyResult, len(e.fuzzyFiles))
		for i, f := range e.fuzzyFiles {
			e.fuzzyResults[i] = fuzzyResult{path: f.path, fileIdx: i}
		}
		e.fuzzyIdx = 0
		e.fuzzyOff = 0
		e.fuzzyPrevQuery = ""
		e.fuzzyPrevCandidates = nil
		e.msg("fuzzy: " + strconv.Itoa(len(e.fuzzyFiles)) + " files")
		return
	}

	isExtending := strings.HasPrefix(query, e.fuzzyPrevQuery) && e.fuzzyPrevQuery != ""
	e.fuzzyPrevQuery = query

	const maxResults = 200
	q := []rune(strings.ToLower(query))
	var results []fuzzyResult
	minScore := -1

	tryAdd := func(idx int) bool {
		fi := e.fuzzyFiles[idx]
		s, m, ok := fuzzyScoreRunes(q, fi.lower, fi.path)
		if !ok {
			return false
		}
		if !(len(results) >= maxResults && s < minScore) {
			pos := sort.Search(len(results), func(i int) bool {
				if results[i].score != s {
					return results[i].score < s
				}
				return results[i].path >= fi.path
			})
			n := len(results)
			results = append(results, fuzzyResult{})
			if pos < n {
				copy(results[pos+1:], results[pos:n])
			}
			results[pos] = fuzzyResult{path: fi.path, score: s, matches: m, fileIdx: idx}
			if len(results) > maxResults {
				results = results[:maxResults]
			}
			if len(results) >= maxResults {
				minScore = results[maxResults-1].score
			}
		}
		return true
	}

	if isExtending && len(e.fuzzyPrevCandidates) > 0 && len(e.fuzzyPrevCandidates) < 3000 {
		matched := make([]int, 0, len(e.fuzzyPrevCandidates))
		for _, idx := range e.fuzzyPrevCandidates {
			if tryAdd(idx) {
				matched = append(matched, idx)
			}
		}
		e.fuzzyPrevCandidates = matched
	} else {
		matched := make([]int, 0, len(e.fuzzyFiles))
		for idx := range e.fuzzyFiles {
			if tryAdd(idx) {
				matched = append(matched, idx)
			}
		}
		e.fuzzyPrevCandidates = matched
	}

	e.fuzzyResults = results
	if e.fuzzyIdx >= len(e.fuzzyResults) {
		e.fuzzyIdx = 0
	}
	e.fuzzyOff = 0
	e.msg("fuzzy: " + strconv.Itoa(len(e.fuzzyResults)) + " matches for \"" + query + "\"")
}

func (e *Editor) fuzzyOpen() {
	if len(e.fuzzyResults) == 0 {
		e.msg("fuzzy: no matches to open")
		return
	}
	path := filepath.Join(e.initialDir, e.fuzzyResults[e.fuzzyIdx].path)
	e.fuzzyCancel()
	e.mode = "editor"
	e.app.SetFocus(e.editorBox)
	e.loadFile(path)	
}

func (e *Editor) fuzzyUp() {
	if e.fuzzyIdx > 0 {
		e.fuzzyIdx--
	}
	if e.fuzzyIdx < e.fuzzyOff {
		e.fuzzyOff = e.fuzzyIdx
	}
}

func (e *Editor) fuzzyDown() {
	if e.fuzzyIdx < len(e.fuzzyResults)-1 {
		e.fuzzyIdx++
	}
	maxResults := 12
	if e.fuzzyIdx >= e.fuzzyOff+maxResults {
		e.fuzzyOff = e.fuzzyIdx - maxResults + 1
	}
}

func (e *Editor) fuzzyPgUp() {
	maxResults := 12
	e.fuzzyIdx -= maxResults
	if e.fuzzyIdx < 0 {
		e.fuzzyIdx = 0
	}
	e.fuzzyOff = e.fuzzyIdx
}

func (e *Editor) fuzzyPgDn() {
	maxResults := 12
	last := len(e.fuzzyResults) - 1
	e.fuzzyIdx += maxResults
	if e.fuzzyIdx > last {
		e.fuzzyIdx = last
	}
	if e.fuzzyIdx >= e.fuzzyOff+maxResults {
		e.fuzzyOff = e.fuzzyIdx - maxResults + 1
	}
}

func (e *Editor) fuzzyHome() {
	e.fuzzyIdx = 0
	e.fuzzyOff = 0
}

func (e *Editor) fuzzyEnd() {
	e.fuzzyIdx = len(e.fuzzyResults) - 1
	maxResults := 12
	if e.fuzzyIdx >= e.fuzzyOff+maxResults {
		e.fuzzyOff = e.fuzzyIdx - maxResults + 1
	}
}
