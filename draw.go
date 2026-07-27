package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
)

func (e *Editor) drawSidebar(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
	for dy := 0; dy < height; dy++ {
		for dx := 0; dx < width; dx++ {
			screen.SetContent(x+dx, y+dy, ' ', nil, tcell.StyleDefault.Background(colMantle))
		}
	}

	header := " Files "
	for dx, ch := range header {
		if dx < width {
			screen.SetContent(x+dx, y, ch, nil, tcell.StyleDefault.Background(colSurface0).Foreground(colBlue))
		}
	}

	rows := height - 1
	for i := 0; i < rows && i+e.sidebarOff < len(e.sidebarFiles); i++ {
		idx := i + e.sidebarOff
		name := e.sidebarFiles[idx]
		rowY := y + i + 1
		disp := " " + name
		if len(disp) > width {
			disp = disp[:width]
		}
		disp += strings.Repeat(" ", width-len(disp))

		gitFg, hasGit := e.sidebarGitColor(name)
		isDir := len(name) > 0 && name[len(name)-1] == '/'
		if e.mode == "sidebar" && idx == e.sidebarIdx {
			fg := colText
			if hasGit {
				fg = gitFg
			} else if isDir {
				fg = colBlue
			}
			for dx, ch := range disp {
				screen.SetContent(x+dx, rowY, ch, nil, tcell.StyleDefault.Background(colSurface1).Foreground(fg))
			}
		} else {
			fg := colSubtext0
			if hasGit {
				fg = gitFg
			} else if isDir {
				fg = colBlue
			}
			for dx, ch := range disp {
				screen.SetContent(x+dx, rowY, ch, nil, tcell.StyleDefault.Background(colMantle).Foreground(fg))
			}
		}
	}

	return x, y, width, height
}

func (e *Editor) drawEditor(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
	if e.gitDirty {
		e.gitDirty = false
		e.updateGitLineStat()
	}

	e.tokenizeBuffer()

	sw := 0
	editX := x
	if e.showSidebar {
		sw = e.sidebarWidth
		editX = sw + 1
	}
	editW := width - (editX - x)

	tabH := 1
	for dx := 0; dx < width; dx++ {
		screen.SetContent(x+dx, y, ' ', nil, tcell.StyleDefault.Background(colSurface0))
	}
	tabsX := x
	for i := e.tabOffset; i < len(e.openFiles); i++ {
		t := e.openFiles[i]
		label := filepath.Base(t.filename)
		if t.filename == "" {
			label = "untitled"
		}
		if t.modified {
			label += " •"
		}
		seg := " " + label + " "
		segRunes := []rune(seg)
		if tabsX+len(segRunes) > x+width {
			break
		}
		bg := colSurface0
		fg := colSubtext0
		if i == e.activeTab {
			bg = colBlue
			fg = colBase
		}
		for d, r := range segRunes {
			screen.SetContent(tabsX+d, y, r, nil, tcell.StyleDefault.Background(bg).Foreground(fg))
		}
		tabsX += len(segRunes)
		if tabsX < x+width {
			screen.SetContent(tabsX, y, ' ', nil, tcell.StyleDefault.Background(bg))
			tabsX++
		}
	}

	contentY := y + tabH
	contentH := height - tabH

	lnW := e.maxLineNumW()
	gutterW := lnW + 3

	searchQlen := len([]rune(e.searchQuery))
	type matchRange struct{ start, end int; active bool }
	searchByLine := make(map[int][]matchRange)
	for mi, m := range e.searchMatches {
		searchByLine[m.Y] = append(searchByLine[m.Y], matchRange{
			start:  m.X,
			end:    m.X + searchQlen,
			active: mi == e.searchIdx,
		})
	}

	for dy := 0; dy < contentH; dy++ {
		by := dy + e.offset.Y
		screenY := contentY + dy

		if by >= len(e.buffer) {
			break
		}

		for gx := 0; gx < gutterW && gx < editW; gx++ {
			screen.SetContent(editX+gx, screenY, ' ', nil, tcell.StyleDefault.Background(colBase).Foreground(colOverlay0))
		}
		if by < len(e.openFiles[e.activeTab].gitLineStat) {
			switch e.openFiles[e.activeTab].gitLineStat[by] {
			case '~':
				screen.SetContent(editX, screenY, '▎', nil, tcell.StyleDefault.Background(colBase).Foreground(colNumber))
			case '+':
				screen.SetContent(editX, screenY, '▎', nil, tcell.StyleDefault.Background(colBase).Foreground(colGreen))
			}
		}
		lnStr := fmt.Sprintf("%*d ", lnW, by+1)
		for gi, ch := range lnStr {
			if gi+1 >= gutterW {
				break
			}
			screen.SetContent(editX+gi+1, screenY, ch, nil, tcell.StyleDefault.Background(colBase).Foreground(colOverlay0))
		}

		line := []rune(e.buffer[by])
		textStart := editX + gutterW
		bufX := e.offset.X
		for dispX := 0; dispX < editW-gutterW && bufX < len(line); {
			ch := line[bufX]
			fg := e.tokenColorAt(by, bufX)
			st := tcell.StyleDefault.Background(colBase).Foreground(fg)
			if e.selection.Active && e.inSelection(Point{X: bufX, Y: by}) {
				st = tcell.StyleDefault.Background(colSurface1).Foreground(fg)
			}
			if searchQlen > 0 {
				if infos, ok := searchByLine[by]; ok {
					for _, info := range infos {
						if bufX >= info.start && bufX < info.end {
							if info.active {
								st = st.Background(colKeyword).Foreground(colBase)
							} else {
								st = st.Background(colSurface2).Foreground(fg)
							}
							break
						}
					}
				}
			}
			if ch == '\t' {
				tabW := 4 - (dispX % 4)
				end := dispX + tabW
				if end > editW-gutterW {
					end = editW - gutterW
				}
				for dispX < end {
					screen.SetContent(textStart+dispX, screenY, ' ', nil, st)
					dispX++
				}
				bufX++
			} else {
				screen.SetContent(textStart+dispX, screenY, ch, nil, st)
				dispX++
				bufX++
			}
		}
		if e.selection.Active && e.inSelection(Point{X: len(line), Y: by}) {
			fillStart := textStart + bufToDisp(line, len(line)) - bufToDisp(line, e.offset.X)
			if fillStart < textStart {
				fillStart = textStart
			}
			if fillStart > textStart+editW-gutterW {
				fillStart = textStart + editW - gutterW
			}
			for fx := fillStart; fx < textStart+editW-gutterW; fx++ {
				screen.SetContent(fx, screenY, ' ', nil, tcell.StyleDefault.Background(colSurface1))
			}
		}
	}

	if e.mode == "editor" && e.cursor.Y >= 0 && e.cursor.Y < len(e.buffer) {
		line := []rune(e.buffer[e.cursor.Y])
		cx := editX + gutterW + bufToDisp(line, e.cursor.X) - bufToDisp(line, e.offset.X)
		cy := contentY + e.cursor.Y - e.offset.Y
		if cx >= editX && cx < editX+editW && cy >= y && cy < y+height {
			screen.ShowCursor(cx, cy)
		}
	} else {
		screen.HideCursor()
	}

	if e.showHelp {
		e.drawHelp(screen)
		screen.HideCursor()
		return x, y, width, height
	}

	if e.showFuzzy {
		screen.HideCursor()
		e.drawFuzzyFinder(screen)
	}

	if e.showTextSearch {
		screen.HideCursor()
		e.drawTextSearch(screen)
	}

	if e.showThemePicker {
		screen.HideCursor()
		e.drawThemePicker(screen)
	}

	return x, y, width, height
}

func (e *Editor) drawHelp(screen tcell.Screen) {
	w, h := screen.Size()

	margin := 4
	boxW := w - margin*2
	boxH := h - margin*2
	if boxW > 64 {
		boxW = 64
	}
	if boxH < 10 {
		boxH = 10
	}
	boxX := (w - boxW) / 2
	boxY := (h - boxH) / 2

	topLeft, topRight, botLeft, botRight := '╭', '╮', '╰', '╯'
	hoz, vert := '─', '│'

	boxBg := colSurface0

	for dy := 0; dy < boxH; dy++ {
		for dx := 0; dx < boxW; dx++ {
			sx := boxX + dx
			sy := boxY + dy
			if sx < 0 || sx >= w || sy < 0 || sy >= h {
				continue
			}
			var ch rune
			switch {
			case dy == 0 && dx == 0:
				ch = topLeft
			case dy == 0 && dx == boxW-1:
				ch = topRight
			case dy == boxH-1 && dx == 0:
				ch = botLeft
			case dy == boxH-1 && dx == boxW-1:
				ch = botRight
			case dy == 0 || dy == boxH-1:
				ch = hoz
			case dx == 0 || dx == boxW-1:
				ch = vert
			}
			fg := colOverlay0
			if ch == 0 {
				fg = colText
			}
			st := tcell.StyleDefault.Background(boxBg).Foreground(fg)
			screen.SetContent(sx, sy, ch, nil, st)
		}
	}

	maxLines := boxH - 2
	if maxLines <= 0 {
		return
	}
	scrollable := len(e.helpLines) > maxLines
	maxOff := len(e.helpLines) - maxLines
	if e.helpOffset > maxOff {
		e.helpOffset = maxOff
	}
	if e.helpOffset < 0 {
		e.helpOffset = 0
	}

	for i := 0; i < maxLines && i+e.helpOffset < len(e.helpLines); i++ {
		line := e.helpLines[i+e.helpOffset]
		sy := boxY + 1 + i
		for cx, ch := range line {
			sx := boxX + 1 + cx
			if sx >= boxX+boxW-1 {
				break
			}
			trim := strings.TrimSpace(line)
			fg := colText
			switch {
			case trim == "KEYBOARD SHORTCUTS":
				fg = colBlue
			case strings.HasPrefix(line, "  ─"):
				fg = colOverlay0
			case strings.Contains(line, "scroll"):
				fg = colSubtext0
			case strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && trim != "":
				fg = colGreen
			}
			st := tcell.StyleDefault.Background(colSurface0).Foreground(fg)
			screen.SetContent(sx, sy, ch, nil, st)
		}
	}

	if scrollable {
		scroll := fmt.Sprintf(" %d/%d ", e.helpOffset+1, maxOff+1)
		sx := boxX + boxW - 2 - len(scroll)
		sy := boxY + boxH - 1
		for cx, ch := range scroll {
			st := tcell.StyleDefault.Background(boxBg).Foreground(colSubtext0)
			screen.SetContent(sx+cx, sy, ch, nil, st)
		}
	}
}

func (e *Editor) drawStatusBar(screen tcell.Screen) {
	w, h := screen.Size()
	statusY := h - 1

	for cx := 0; cx < w; cx++ {
		screen.SetContent(cx, statusY, ' ', nil, tcell.StyleDefault.Background(colSurface1))
	}

	modeTag := " EDITOR "
	modeBg := colSurface1
	modeFg := colText
	if e.mode == "sidebar" {
		modeTag = " SIDEBAR "
		modeBg = colBlue
		modeFg = colBase
	}
	for cx, ch := range modeTag {
		screen.SetContent(cx, statusY, ch, nil, tcell.StyleDefault.Background(modeBg).Foreground(modeFg))
	}

	shortcuts := ""

	branchTag := ""
	if e.git != nil && e.git.branch != "" {
		branchTag = " @" + e.git.branch + " "
	}
	right := branchTag + fmt.Sprintf("%d/%d  %d:%d ", e.cursor.Y+1, max(1, len(e.buffer)), e.cursor.Y+1, e.cursor.X+1)

	modeEnd := len(modeTag)
	nameX := modeEnd + 1

	name := "[No Name]"
	if e.filename != "" {
		name = filepath.Base(e.filename)
	}
	if e.modified {
		name += " •"
	}

	rightW := len(right)
	availW := w - nameX - rightW

	shortcutsX := nameX
	if availW > len(name)+2 {
		for cx, ch := range name {
			sx := nameX + cx
			if sx >= w {
				break
			}
			screen.SetContent(sx, statusY, ch, nil, tcell.StyleDefault.Background(colSurface1).Foreground(colText))
		}
		shortcutsX = nameX + len(name)
	}

	maxShortcutW := (w - rightW) - shortcutsX - len(" F1 help ")
	for cx, ch := range shortcuts {
		if cx >= maxShortcutW {
			break
		}
		sx := shortcutsX + cx
		st := tcell.StyleDefault.Background(colSurface1).Foreground(colSubtext0)
		if ch == '│' {
			st = tcell.StyleDefault.Background(colSurface1).Foreground(colOverlay0)
		}
		screen.SetContent(sx, statusY, ch, nil, st)
	}

	if e.msgTimer > 0 && e.message != "" {
		msg := " " + e.message + " "
		msgStart := shortcutsX + 1
		msgEnd := (w - rightW) - len(" F1 help ")
		if msgStart < msgEnd {
			for cx, ch := range msg {
				sx := msgStart + cx
				if sx >= msgEnd {
					break
				}
				screen.SetContent(sx, statusY, ch, nil, tcell.StyleDefault.Background(colGreen).Foreground(colBase))
			}
		}
		e.msgTimer--
	}

	helpHint := " F1 help "
	helpX := (w - rightW) - len(helpHint)
	for cx, ch := range helpHint {
		sx := helpX + cx
		st := tcell.StyleDefault.Background(colSurface1).Foreground(colGreen)
		if ch == ' ' {
			st = tcell.StyleDefault.Background(colSurface1).Foreground(colSurface1)
		}
		screen.SetContent(sx, statusY, ch, nil, st)
	}

	for cx, ch := range right {
		sx := (w - rightW) + cx
		if sx >= w {
			break
		}
		screen.SetContent(sx, statusY, ch, nil, tcell.StyleDefault.Background(colSurface1).Foreground(colSubtext0))
	}
}

func (e *Editor) scrollCursor() {
	_, _, width, height := e.editorBox.GetRect()
	ew := width
	if e.showSidebar {
		ew--
	}
	lnW := e.maxLineNumW() + 3
	ew -= lnW
	if ew < 1 {
		ew = 1
	}

	if e.cursor.Y < e.offset.Y {
		e.offset.Y = e.cursor.Y
	}
	ch := height - 1
	if e.cursor.Y >= e.offset.Y+ch {
		e.offset.Y = e.cursor.Y - ch + 1
	}
	if e.cursor.X < e.offset.X {
		e.offset.X = e.cursor.X
	}
	line := []rune(e.buffer[e.cursor.Y])
	cursorDisp := bufToDisp(line, e.cursor.X)
	offsetDisp := bufToDisp(line, e.offset.X)
	if cursorDisp < offsetDisp {
		e.offset.X = dispToBuf(line, cursorDisp)
	}
	if cursorDisp >= offsetDisp+ew {
		wantDisp := cursorDisp - ew + 1
		e.offset.X = dispToBuf(line, wantDisp)
	}
}

func bufToDisp(line []rune, bufX int) int {
	col := 0
	for i := 0; i < bufX && i < len(line); i++ {
		if line[i] == '\t' {
			col += 4 - (col % 4)
		} else {
			col++
		}
	}
	return col
}

func dispToBuf(line []rune, dispX int) int {
	col := 0
	for i := 0; i < len(line); i++ {
		if line[i] == '\t' {
			next := col + 4 - (col % 4)
			if next > dispX {
				return i
			}
			col = next
		} else {
			if col >= dispX {
				return i
			}
			col++
		}
	}
	return len(line)
}

func (e *Editor) drawFuzzyFinder(screen tcell.Screen) {
	w, h := screen.Size()

	maxResults := 12
	count := len(e.fuzzyResults)
	if count > maxResults {
		count = maxResults
	}
	boxW := w - 4
	if boxW < 20 {
		boxW = 20
	}
	boxH := count + 3
	if boxH < 4 {
		boxH = 4
	}
	boxX := (w - boxW) / 2
	boxY := h - boxH - 2

	if boxY < 0 {
		boxY = 0
	}

	for dy := 0; dy < boxH; dy++ {
		for dx := 0; dx < boxW; dx++ {
			sx := boxX + dx
			sy := boxY + dy
			if sx < 0 || sx >= w || sy < 0 || sy >= h {
				continue
			}
			ch := ' '
			fg := colText
			bg := colSurface0
			switch {
			case dy == 0 && dx == 0:
				ch = '╭'
				fg = colOverlay0
			case dy == 0 && dx == boxW-1:
				ch = '╮'
				fg = colOverlay0
			case dy == boxH-1 && dx == 0:
				ch = '╰'
				fg = colOverlay0
			case dy == boxH-1 && dx == boxW-1:
				ch = '╯'
				fg = colOverlay0
			case dy == 0 || dy == boxH-1:
				ch = '─'
				fg = colOverlay0
			case dx == 0 || dx == boxW-1:
				ch = '│'
				fg = colOverlay0
			}
			screen.SetContent(sx, sy, ch, nil, tcell.StyleDefault.Background(bg).Foreground(fg))
		}
	}

	// Query line at dy=1
	query := "> " + e.fuzzyQuery
	if len(query) > boxW-2 {
		query = query[:boxW-2]
	}
	query += strings.Repeat(" ", boxW-2-len(query))
	qy := boxY + 1
	for dx, ch := range query {
		screen.SetContent(boxX+1+dx, qy, ch, nil, tcell.StyleDefault.Background(colSurface0).Foreground(colGreen))
	}

	// Results start at dy=2
	maxPathLen := boxW - 4
	start := e.fuzzyOff
	resultRows := 0
	for i := 0; i < maxResults && i+start < len(e.fuzzyResults); i++ {
		r := e.fuzzyResults[i+start]
		path := r.path
		truncOffset := 0
		visiblePath := path
		if len(path) > maxPathLen {
			keepLen := maxPathLen - 3
			truncOffset = len(path) - keepLen
			visiblePath = "..." + path[truncOffset:]
		}

		sy := boxY + 2 + i
		isSel := (i + start) == e.fuzzyIdx

		// Build match lookup for this path
		isMatch := make(map[int]bool)
		for _, mi := range r.matches {
			isMatch[mi] = true
		}

		// Draw character by character
		muted := colOverlay0
		highlightFg := colText
		bg := colSurface0
		if isSel {
			bg = colBlue
			highlightFg = colBase
		}
		dx := 0
		skipDots := 0
		if truncOffset > 0 {
			skipDots = 3
		}
		for _, ch := range visiblePath {
			fg := muted
			if dx >= skipDots {
				origIdx := truncOffset + dx - skipDots
				if isMatch[origIdx] {
					fg = highlightFg
				}
			}
			if dx < boxW-2 {
				screen.SetContent(boxX+1+dx, sy, ch, nil, tcell.StyleDefault.Background(bg).Foreground(fg))
			}
			dx++
		}
		// Pad with spaces
		for dx < boxW-2 {
			screen.SetContent(boxX+1+dx, sy, ' ', nil, tcell.StyleDefault.Background(bg))
			dx++
		}
		resultRows++
	}

	// Scroll indicator on right border
	total := len(e.fuzzyResults)
	if total > resultRows {
		contentH := boxH - 3
		if contentH > 0 {
			scrollable := total - resultRows
			thumbPos := 0
			if scrollable > 0 {
				thumbPos = (e.fuzzyOff * contentH) / scrollable
			}
			if thumbPos >= contentH {
				thumbPos = contentH - 1
			}
			sy := boxY + 2 + thumbPos
			screen.SetContent(boxX+boxW-1, sy, '▓', nil, tcell.StyleDefault.Background(colSurface0).Foreground(colOverlay0))
		}
	}
}

func (e *Editor) drawTextSearch(screen tcell.Screen) {
	w, h := screen.Size()

	const contextLines = 1
	const resultRows = 5

	maxResults := e.textSearchRowCount()
	visibleCount := len(e.textSearchResults)
	if visibleCount > maxResults {
		visibleCount = maxResults
	}
	headerRows := 2
	footerRows := 1
	boxH := visibleCount*resultRows + headerRows + footerRows
	if boxH < headerRows+footerRows {
		boxH = headerRows + footerRows
	}
	if boxH > h-2 {
		boxH = h - 2
	}
	maxContent := (boxH - headerRows - footerRows) / resultRows
	if visibleCount > maxContent {
		visibleCount = maxContent
	}

	boxW := w - 4
	if boxW < 40 {
		boxW = 40
	}
	boxX := (w - boxW) / 2
	boxY := h - boxH - 2
	if boxY < 0 {
		boxY = 0
	}

	if e.textSearchIdx < e.textSearchOff {
		e.textSearchOff = e.textSearchIdx
	}
	if e.textSearchIdx >= e.textSearchOff+visibleCount && visibleCount > 0 {
		e.textSearchOff = e.textSearchIdx - visibleCount + 1
	}

	contentW := boxW - 2
	innerW := contentW - 2

	for dy := 0; dy < boxH; dy++ {
		for dx := 0; dx < boxW; dx++ {
			sx := boxX + dx
			sy := boxY + dy
			if sx < 0 || sx >= w || sy < 0 || sy >= h {
				continue
			}
			ch := ' '
			fg := colText
			bg := colSurface0
			switch {
			case dy == 0 && dx == 0:
				ch = '╭'
				fg = colOverlay0
			case dy == 0 && dx == boxW-1:
				ch = '╮'
				fg = colOverlay0
			case dy == boxH-1 && dx == 0:
				ch = '╰'
				fg = colOverlay0
			case dy == boxH-1 && dx == boxW-1:
				ch = '╯'
				fg = colOverlay0
			case dy == 0 || dy == boxH-1:
				ch = '─'
				fg = colOverlay0
			case dx == 0 || dx == boxW-1:
				ch = '│'
				fg = colOverlay0
			}
			screen.SetContent(sx, sy, ch, nil, tcell.StyleDefault.Background(bg).Foreground(fg))
		}
	}

	query := "> " + e.textSearchQuery
	if len([]rune(query)) > contentW {
		query = string([]rune(query)[:contentW])
	}
	query += strings.Repeat(" ", contentW-len([]rune(query)))
	for dx, ch := range query {
		screen.SetContent(boxX+1+dx, boxY+1, ch, nil, tcell.StyleDefault.Background(colSurface0).Foreground(colGreen))
	}

	curY := boxY + 2
	queryLen := len([]rune(e.textSearchQuery))

	for i := 0; i+e.textSearchOff < len(e.textSearchResults) && i < visibleCount && curY+resultRows <= boxY+boxH-1; i++ {
		r := e.textSearchResults[i+e.textSearchOff]
		idx := i + e.textSearchOff
		isSel := idx == e.textSearchIdx
		borderFg := colOverlay0
		if isSel {
			borderFg = colBlue
		}

		if isSel && innerW > 0 {
			lineLen := len([]rune(r.line))
			desiredPos := innerW / 3
			newOff := r.matchCol - desiredPos
			if newOff < 0 {
				newOff = 0
			}
			if lineLen > innerW && newOff > lineLen-innerW {
				newOff = lineLen - innerW
			}
			if lineLen <= innerW {
				newOff = 0
			}
			e.textSearchHOff = newOff
		}

		hOff := 0
		if isSel {
			hOff = e.textSearchHOff
		}

		// Top border: ┌── file.go:42 ──────────┐
		label := r.filePath + ":" + fmt.Sprint(r.lineNum)
		labelRunes := []rune(label)
		topRunes := make([]rune, innerW)
		topRunes[0] = '┌'
		topRunes[innerW-1] = '┐'
		for dx := 1; dx < innerW-1; dx++ {
			labelIdx := dx - 1
			if labelIdx >= 0 && labelIdx < len(labelRunes) {
				topRunes[dx] = labelRunes[labelIdx]
			} else {
				topRunes[dx] = '─'
			}
		}
		for dx := 0; dx < innerW; dx++ {
			screen.SetContent(boxX+1+dx, curY, topRunes[dx], nil, tcell.StyleDefault.Background(colSurface0).Foreground(borderFg))
		}
		curY++

		// draw a content line with left/right borders
		drawLine := func(text string, fg tcell.Color) {
			textRunes := []rune(text)
			screen.SetContent(boxX+1, curY, '│', nil, tcell.StyleDefault.Background(colSurface0).Foreground(borderFg))
			for dx := 1; dx < innerW-1; dx++ {
				bufIdx := hOff + dx - 1
				ch := ' '
				if bufIdx >= 0 && bufIdx < len(textRunes) {
					ch = textRunes[bufIdx]
				}
				screen.SetContent(boxX+1+dx, curY, ch, nil, tcell.StyleDefault.Background(colSurface0).Foreground(fg))
			}
			screen.SetContent(boxX+innerW, curY, '│', nil, tcell.StyleDefault.Background(colSurface0).Foreground(borderFg))
			curY++
		}

		// Context above
		if len(r.before) > contextLines {
			r.before = r.before[len(r.before)-contextLines:]
		}
		for _, bl := range r.before {
			drawLine(bl, colSubtext0)
		}
		for pad := len(r.before); pad < contextLines; pad++ {
			drawLine("", colSubtext0)
		}

		// Match line
		matchRunes := []rune(r.line)
		screen.SetContent(boxX+1, curY, '│', nil, tcell.StyleDefault.Background(colSurface0).Foreground(borderFg))
		for dx := 1; dx < innerW-1; dx++ {
			bufIdx := hOff + dx - 1
			ch := ' '
			if bufIdx >= 0 && bufIdx < len(matchRunes) {
				ch = matchRunes[bufIdx]
			}
			if queryLen > 0 && bufIdx >= r.matchCol && bufIdx < r.matchCol+queryLen {
				screen.SetContent(boxX+1+dx, curY, ch, nil, tcell.StyleDefault.Background(colKeyword).Foreground(colBase))
			} else {
				screen.SetContent(boxX+1+dx, curY, ch, nil, tcell.StyleDefault.Background(colSurface0).Foreground(colText))
			}
		}
		screen.SetContent(boxX+innerW, curY, '│', nil, tcell.StyleDefault.Background(colSurface0).Foreground(borderFg))
		curY++

		// Context below
		if len(r.after) > contextLines {
			r.after = r.after[:contextLines]
		}
		for _, al := range r.after {
			drawLine(al, colSubtext0)
		}
		for pad := len(r.after); pad < contextLines; pad++ {
			drawLine("", colSubtext0)
		}

		// Bottom border
		screen.SetContent(boxX+1, curY, '└', nil, tcell.StyleDefault.Background(colSurface0).Foreground(borderFg))
		for dx := 1; dx < innerW-1; dx++ {
			screen.SetContent(boxX+1+dx, curY, '─', nil, tcell.StyleDefault.Background(colSurface0).Foreground(borderFg))
		}
		screen.SetContent(boxX+innerW, curY, '┘', nil, tcell.StyleDefault.Background(colSurface0).Foreground(borderFg))
		curY++
	}

	total := len(e.textSearchResults)
	visibleRows := (boxH - 3) / resultRows
	if total > visibleRows {
		contentHInner := boxH - 3
		scrollable := total - visibleRows
		thumbPos := 0
		if scrollable > 0 {
			thumbPos = (e.textSearchOff * contentHInner) / scrollable
		}
		if thumbPos >= contentHInner {
			thumbPos = contentHInner - 1
		}
		sy := boxY + 2 + thumbPos
		screen.SetContent(boxX+boxW-1, sy, '▓', nil, tcell.StyleDefault.Background(colSurface0).Foreground(colOverlay0))
	}
}

func (e *Editor) drawThemePicker(screen tcell.Screen) {
	w, h := screen.Size()

	listH := len(themes)
	rows := e.themePickerRowCount()
	if rows > listH {
		rows = listH
	}
	boxH := rows + 4
	if boxH > h-2 {
		boxH = h - 2
	}
	boxW := 40
	if boxW > w-4 {
		boxW = w - 4
	}
	boxX := (w - boxW) / 2
	boxY := (h - boxH) / 2

	if e.themePickerIdx < e.themePickerOff {
		e.themePickerOff = e.themePickerIdx
	}
	if e.themePickerIdx >= e.themePickerOff+rows && rows > 0 {
		e.themePickerOff = e.themePickerIdx - rows + 1
	}

	for dy := 0; dy < boxH; dy++ {
		for dx := 0; dx < boxW; dx++ {
			sx := boxX + dx
			sy := boxY + dy
			if sx < 0 || sx >= w || sy < 0 || sy >= h {
				continue
			}
			ch := ' '
			fg := colText
			bg := colSurface0
			switch {
			case dy == 0 && dx == 0:
				ch = '╭'
				fg = colOverlay0
			case dy == 0 && dx == boxW-1:
				ch = '╮'
				fg = colOverlay0
			case dy == boxH-1 && dx == 0:
				ch = '╰'
				fg = colOverlay0
			case dy == boxH-1 && dx == boxW-1:
				ch = '╯'
				fg = colOverlay0
			case dy == 0 || dy == boxH-1:
				ch = '─'
				fg = colOverlay0
			case dx == 0 || dx == boxW-1:
				ch = '│'
				fg = colOverlay0
			}
			screen.SetContent(sx, sy, ch, nil, tcell.StyleDefault.Background(bg).Foreground(fg))
		}
	}

	title := " Themes "
	titleRunes := []rune(title)
	for dx, ch := range titleRunes {
		screen.SetContent(boxX+1+dx, boxY+1, ch, nil, tcell.StyleDefault.Background(colSurface0).Foreground(colBlue))
	}

	contentW := boxW - 2
	for i := 0; i < rows && i+e.themePickerOff < len(themes); i++ {
		idx := i + e.themePickerOff
		name := themes[idx].Name
		sy := boxY + 2 + i

		if idx == e.themePickerIdx {
			for dx := 0; dx < contentW; dx++ {
				ch := ' '
				fg := colBase
				if dx < len(name) {
					ch = rune(name[dx])
				}
				screen.SetContent(boxX+1+dx, sy, ch, nil, tcell.StyleDefault.Background(colBlue).Foreground(fg))
			}
		} else {
			for dx := 0; dx < contentW; dx++ {
				ch := ' '
				fg := colText
				if dx < len(name) {
					ch = rune(name[dx])
				}
				screen.SetContent(boxX+1+dx, sy, ch, nil, tcell.StyleDefault.Background(colSurface0).Foreground(fg))
			}
		}
	}

	total := len(themes)
	if total > rows {
		contentHInner := boxH - 4
		scrollable := total - rows
		thumbPos := 0
		if scrollable > 0 {
			thumbPos = (e.themePickerOff * contentHInner) / scrollable
		}
		if thumbPos >= contentHInner {
			thumbPos = contentHInner - 1
		}
		sy := boxY + 2 + thumbPos
		screen.SetContent(boxX+boxW-1, sy, '▓', nil, tcell.StyleDefault.Background(colSurface0).Foreground(colOverlay0))
	}

	// instruction line
	help := "↑↓ browse  ·  Enter select  ·  Esc cancel"
	helpRunes := []rune(help)
	helpY := boxY + boxH - 1
	for dx, ch := range helpRunes {
		sx := boxX + 1 + dx
		if sx >= boxX+boxW-1 {
			break
		}
		screen.SetContent(sx, helpY, ch, nil, tcell.StyleDefault.Background(colSurface0).Foreground(colSubtext0))
	}
}
