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

		if e.mode == "sidebar" && idx == e.sidebarIdx {
			for dx, ch := range disp {
				screen.SetContent(x+dx, rowY, ch, nil, tcell.StyleDefault.Background(colSurface1).Foreground(colText))
			}
		} else {
			for dx, ch := range disp {
				screen.SetContent(x+dx, rowY, ch, nil, tcell.StyleDefault.Background(colMantle).Foreground(colSubtext0))
			}
		}
	}

	return x, y, width, height
}

func (e *Editor) drawEditor(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
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
	for i, t := range e.openFiles {
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
	gutterW := lnW + 2

	for dy := 0; dy < contentH; dy++ {
		by := dy + e.offset.Y
		screenY := contentY + dy

		if by >= len(e.buffer) {
			break
		}

		for gx := 0; gx < gutterW && gx < editW; gx++ {
			screen.SetContent(editX+gx, screenY, ' ', nil, tcell.StyleDefault.Background(colBase).Foreground(colOverlay0))
		}
		lnStr := fmt.Sprintf("%*d ", lnW, by+1)
		for gi, ch := range lnStr {
			if gi >= gutterW {
				break
			}
			screen.SetContent(editX+gi, screenY, ch, nil, tcell.StyleDefault.Background(colBase).Foreground(colOverlay0))
		}

		line := []rune(e.buffer[by])
		textStart := editX + gutterW
		searchQlen := len([]rune(e.searchQuery))
		for dx := 0; dx < editW-gutterW && dx+e.offset.X < len(line); dx++ {
			ch := line[dx+e.offset.X]
			fg := e.tokenColorAt(by, dx+e.offset.X)
			st := tcell.StyleDefault.Background(colBase).Foreground(fg)
			if e.selection.Active && e.inSelection(Point{X: dx + e.offset.X, Y: by}) {
				st = tcell.StyleDefault.Background(colSurface1).Foreground(fg)
			}
			if searchQlen > 0 {
				for mi, m := range e.searchMatches {
					if m.Y > by {
						break
					}
					if m.Y == by && dx+e.offset.X >= m.X && dx+e.offset.X < m.X+searchQlen {
						bg := colSurface2
						if mi == e.searchIdx {
							bg = colKeyword
							fg = colBase
						}
						st = st.Background(bg).Foreground(fg)
						break
					}
				}
			}
			screen.SetContent(textStart+dx, screenY, ch, nil, st)
		}
		if e.selection.Active && e.inSelection(Point{X: len(line), Y: by}) {
			fillStart := textStart + clamp(len(line)-e.offset.X, 0, editW-gutterW)
			for fx := fillStart; fx < textStart+editW-gutterW; fx++ {
				screen.SetContent(fx, screenY, ' ', nil, tcell.StyleDefault.Background(colSurface1))
			}
		}
	}

	if e.mode == "editor" {
		cx := editX + gutterW + e.cursor.X - e.offset.X
		cy := contentY + e.cursor.Y - e.offset.Y
		if cx >= editX && cx < editX+editW && cy >= y && cy < y+height {
			screen.ShowCursor(cx, cy)
		}
	} else {
		screen.HideCursor()
	}

	return x, y, width, height
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
	if e.mode == "sidebar" {
		dot := "hide"
		if e.hideDotfiles {
			dot = "show"
		}
		shortcuts = " ^F filter │ ^R " + dot + " dotfiles │ ← up │ → enter │ Del delete │ Esc edit │ Alt←→ tab "
	} else {
		shortcuts = " ^F search │ ^S save │ ^O open │ ^N new │ ^C copy │ ^V paste │ ^X cut │ ^D dup │ ^W close │ Alt←→ tab │ Alt+T theme "
	}

	right := fmt.Sprintf(" %d:%d ", e.cursor.Y+1, e.cursor.X+1)

	modeEnd := len(modeTag)
	nameX := modeEnd + 2

	name := "[No Name]"
	if e.filename != "" {
		name = filepath.Base(e.filename)
	}
	if e.modified {
		name += " •"
	}

	rightW := len(right)
	shortcutsW := len(shortcuts)
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
		shortcutsX = nameX + len(name) + 1
	} else {
		shortcutsX = nameX
	}

	if shortcutsX+shortcutsW+rightW < w {
		for cx, ch := range shortcuts {
			sx := shortcutsX + cx
			if sx >= w {
				break
			}
			st := tcell.StyleDefault.Background(colSurface1).Foreground(colSubtext0)
			if ch == '│' {
				st = tcell.StyleDefault.Background(colSurface1).Foreground(colOverlay0)
			}
			screen.SetContent(sx, statusY, ch, nil, st)
		}
	}

	for cx, ch := range right {
		sx := (w - rightW) + cx
		if sx >= w {
			break
		}
		screen.SetContent(sx, statusY, ch, nil, tcell.StyleDefault.Background(colSurface1).Foreground(colSubtext0))
	}

	if e.msgTimer > 0 && e.message != "" {
		msg := " " + e.message + " "
		msgX := (w - rightW) - len(msg)
		if msgX >= 0 {
			for cx, ch := range msg {
				sx := msgX + cx
				screen.SetContent(sx, statusY, ch, nil, tcell.StyleDefault.Background(colGreen).Foreground(colBase))
			}
		}
		e.msgTimer--
	}
}

func (e *Editor) scrollCursor() {
	_, _, width, height := e.editorBox.GetRect()
	sw := 0
	if e.showSidebar {
		sw = e.sidebarWidth + 2
	}
	ew := width - sw
	lnW := e.maxLineNumW() + 2
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
	if e.cursor.X >= e.offset.X+ew {
		e.offset.X = e.cursor.X - ew + 1
	}
}
