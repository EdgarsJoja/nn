package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/go-git/go-git/v5"
)

type gitInfo struct {
	root   string
	branch string
	status map[string]*git.FileStatus
	dir    string
	repo   *git.Repository
	wt     *git.Worktree
}

type editOp struct {
	op          byte   // '=', '-', '+'
	aLine, bLine int   // line index in headLines / buffer
}

type diffLine struct {
	op     byte   // ' ', '+', '-'
	text   string
	oldNum int    // 1-indexed line in HEAD, 0 for added lines
	newNum int    // 1-indexed line in buffer, 0 for deleted
	hunkIdx int   // index into diffHunks
}

type diffHunk struct {
	oldStart, oldEnd int   // 0-indexed range in headLines
	newStart, newEnd int   // 0-indexed range in buffer
	lines []diffLine
}

func myersDiff(a, b []string) []editOp {
	n, m := len(a), len(b)
	maxD := n + m
	offset := maxD
	v := make([]int, 2*maxD+1)
	var trace [][]int

	for d := 0; d <= maxD; d++ {
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[k-1+offset] < v[k+1+offset]) {
				x = v[k+1+offset]
			} else {
				x = v[k-1+offset] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[k+offset] = x
			if x >= n && y >= m {
				vv := make([]int, len(v))
				copy(vv, v)
				trace = append(trace, vv)
				return myersBacktrack(a, b, trace, offset, n, m)
			}
		}
		vv := make([]int, len(v))
		copy(vv, v)
		trace = append(trace, vv)
	}
	return nil
}

func myersBacktrack(a, b []string, trace [][]int, offset, n, m int) []editOp {
	var ops []editOp
	x, y := n, m

	for d := len(trace) - 1; d >= 1; d-- {
		k := x - y
		prevV := trace[d-1]
		var prevK int
		var wasDeletion bool
		if k == -d || (k != d && prevV[k-1+offset] < prevV[k+1+offset]) {
			prevK = k + 1
			wasDeletion = false
		} else {
			prevK = k - 1
			wasDeletion = true
		}
		prevX := prevV[prevK+offset]
		prevY := prevX - prevK

		for x > prevX && y > prevY {
			x--
			y--
			ops = append(ops, editOp{op: '=', aLine: x, bLine: y})
		}
		if wasDeletion {
			x--
			ops = append(ops, editOp{op: '-', aLine: x})
		} else {
			y--
			ops = append(ops, editOp{op: '+', bLine: y})
		}
	}

	for x > 0 && y > 0 {
		x--
		y--
		ops = append(ops, editOp{op: '=', aLine: x, bLine: y})
	}

	for i, j := 0, len(ops)-1; i < j; i, j = i+1, j-1 {
		ops[i], ops[j] = ops[j], ops[i]
	}
	return ops
}

func computeDiff(headContent string, buffer []string) []diffHunk {
	headLines := strings.Split(headContent, "\n")
	if len(headLines) == 1 && headLines[0] == "" {
		headLines = nil
	}
	buf := buffer
	if len(buf) > 0 && buf[len(buf)-1] == "" && len(headLines) > 0 && headLines[len(headLines)-1] != "" {
		buf = buf[:len(buf)-1]
	}

	ops := myersDiff(headLines, buf)
	if len(ops) == 0 {
		return nil
	}

	headLine := 0
	bufLine := 0
	var hunks []diffHunk
	hunkIdx := 0

	i := 0
	for i < len(ops) {
		if ops[i].op == '=' {
			headLine++
			bufLine++
			i++
			continue
		}

		hunk := diffHunk{
			oldStart: headLine,
			newStart: bufLine,
		}

		const contextLines = 2
		ctxBefore := 0
		for ctxBefore < contextLines && i-ctxBefore-1 >= 0 && ops[i-ctxBefore-1].op == '=' {
			ctxBefore++
		}
		for c := ctxBefore; c > 0; c-- {
			op := ops[i-c]
			hunk.lines = append(hunk.lines, diffLine{
				op:      ' ',
				text:    headLines[op.aLine],
				oldNum:  op.aLine + 1,
				newNum:  op.bLine + 1,
				hunkIdx: hunkIdx,
			})
		}
		hunk.oldStart = headLine - ctxBefore
		hunk.newStart = bufLine - ctxBefore

		for i < len(ops) && ops[i].op != '=' {
			op := ops[i]
			switch op.op {
			case '-':
				hunk.lines = append(hunk.lines, diffLine{
					op:      '-',
					text:    headLines[op.aLine],
					oldNum:  op.aLine + 1,
					newNum:  0,
					hunkIdx: hunkIdx,
				})
				headLine++
			case '+':
				hunk.lines = append(hunk.lines, diffLine{
					op:      '+',
					text:    buf[op.bLine],
					oldNum:  0,
					newNum:  op.bLine + 1,
					hunkIdx: hunkIdx,
				})
				bufLine++
			}
			i++
		}

		ctxAfter := 0
		for ctxAfter < contextLines && i+ctxAfter < len(ops) && ops[i+ctxAfter].op == '=' {
			ctxAfter++
		}
		for c := 0; c < ctxAfter; c++ {
			op := ops[i+c]
			hunk.lines = append(hunk.lines, diffLine{
				op:      ' ',
				text:    headLines[op.aLine],
				oldNum:  op.aLine + 1,
				newNum:  op.bLine + 1,
				hunkIdx: hunkIdx,
			})
			headLine++
			bufLine++
		}

		hunk.oldEnd = headLine - ctxAfter
		hunk.newEnd = bufLine - ctxAfter
		hunks = append(hunks, hunk)
		hunkIdx++
		i += ctxAfter
	}

	return hunks
}

func computeLineStat(headContent string, buffer []string) []byte {
	headLines := strings.Split(headContent, "\n")
	if len(headLines) == 1 && headLines[0] == "" {
		headLines = nil
	}

	buf := buffer
	if len(buf) > 0 && buf[len(buf)-1] == "" && len(headLines) > 0 && headLines[len(headLines)-1] != "" {
		buf = buf[:len(buf)-1]
	}

	stat := make([]byte, len(buf))
	for i := range stat {
		stat[i] = '+'
	}

	ops := myersDiff(headLines, buf)

	for _, op := range ops {
		if op.op == '=' {
			stat[op.bLine] = ' '
		}
	}

	i := 0
	for i < len(ops) {
		if ops[i].op == '=' {
			i++
			continue
		}
		hasDelete := false
		j := i
		for j < len(ops) && ops[j].op != '=' {
			if ops[j].op == '-' {
				hasDelete = true
			}
			j++
		}
		for k := i; k < j; k++ {
			if ops[k].op == '+' {
				if hasDelete {
					stat[ops[k].bLine] = '~'
				}
			}
		}
		i = j
	}

	return stat
}

func (e *Editor) refreshGit() {
	tab := e.activeFile()

	filePath := ""
	if tab != nil {
		filePath = tab.filepath
		if filePath == "" {
			filePath = tab.filename
		}
	}

	var dir string
	if filePath != "" {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return
		}
		dir = filepath.Dir(absPath)
	} else {
		absDir, err := filepath.Abs(e.sidebarDir)
		if err != nil {
			return
		}
		dir = absDir
	}

	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		e.git = nil
		return
	}

	ref, err := repo.Head()
	if err != nil {
		e.git = nil
		return
	}

	wt, err := repo.Worktree()
	if err != nil {
		e.git = nil
		return
	}

	status, err := wt.Status()
	if err != nil {
		e.git = nil
		return
	}

	e.git = &gitInfo{
		root:   wt.Filesystem.Root(),
		branch: ref.Name().Short(),
		status: status,
		dir:    dir,
		repo:   repo,
		wt:     wt,
	}

	e.updateGitFileInfo()
}

func (e *Editor) refreshGitTab() {
	tab := e.activeFile()

	filePath := ""
	if tab != nil {
		filePath = tab.filepath
		if filePath == "" {
			filePath = tab.filename
		}
	}

	var dir string
	if filePath != "" {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return
		}
		dir = filepath.Dir(absPath)
	} else {
		absDir, err := filepath.Abs(e.sidebarDir)
		if err != nil {
			return
		}
		dir = absDir
	}

	if e.git != nil && e.git.repo != nil && e.git.dir == dir {
		e.updateGitFileInfo()
		return
	}

	e.refreshGit()
}

func (e *Editor) updateGitFileInfo() {
	tab := e.activeFile()
	if tab == nil || e.git == nil || e.git.repo == nil {
		return
	}

	filePath := tab.filepath
	if filePath == "" {
		filePath = tab.filename
	}
	if filePath == "" {
		return
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return
	}

	relPath, err := filepath.Rel(e.git.root, absPath)
	if err != nil {
		return
	}
	relPath = filepath.ToSlash(relPath)

	ref, err := e.git.repo.Head()
	if err != nil {
		return
	}
	e.git.branch = ref.Name().Short()

	commit, err := e.git.repo.CommitObject(ref.Hash())
	if err != nil {
		return
	}
	tree, err := commit.Tree()
	if err != nil {
		return
	}

	blob, err := tree.File(relPath)
	if err != nil {
		_, inStatus := e.git.status[relPath]
		if !inStatus {
			tab.gitLineStat = nil
			tab.headContent = ""
			return
		}
		tab.gitLineStat = computeLineStat("", e.buffer)
		tab.headContent = ""
		return
	}
	content, err := blob.Contents()
	if err != nil {
		_, inStatus := e.git.status[relPath]
		if !inStatus {
			tab.gitLineStat = nil
			tab.headContent = ""
			return
		}
		tab.gitLineStat = computeLineStat("", e.buffer)
		tab.headContent = ""
		return
	}

	tab.headContent = content
	tab.gitLineStat = computeLineStat(content, e.buffer)
}

func (e *Editor) updateGitLineStat() {
	tab := e.activeFile()
	if tab == nil || e.git == nil || e.git.repo == nil {
		return
	}
	if tab.headContent == "" && tab.gitLineStat == nil {
		return
	}
	tab.gitLineStat = computeLineStat(tab.headContent, e.buffer)
}

func (e *Editor) sidebarGitColor(name string) (tcell.Color, bool) {
	if e.git == nil || e.git.status == nil {
		return 0, false
	}
	fullPath := filepath.Join(e.sidebarDir, name)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return 0, false
	}
	rel, err := filepath.Rel(e.git.root, absPath)
	if err != nil {
		return 0, false
	}
	rel = filepath.ToSlash(rel)

	s, ok := e.git.status[rel]
	if !ok {
		// File not in git status — could be gitignored but still have local changes
		if _, err := os.Stat(fullPath); err == nil && e.activeTab < len(e.openFiles) && len(e.openFiles[e.activeTab].gitLineStat) > 0 {
			tabPath := e.openFiles[e.activeTab].filepath
			if tabPath == "" {
				tabPath = e.openFiles[e.activeTab].filename
			}
			absTabPath, err := filepath.Abs(tabPath)
			if err == nil {
				tabRel, err := filepath.Rel(e.git.root, absTabPath)
				if err == nil && filepath.ToSlash(tabRel) == rel {
					for _, st := range e.openFiles[e.activeTab].gitLineStat {
						if st != ' ' {
							return colNumber, true
						}
					}
				}
			}
		}
		return 0, false
	}

	// Untracked files should always show red; ignore any gitLineStat override
	if s.Worktree == git.Untracked || s.Staging == git.Untracked {
		return colRed, true
	}

	// For tracked files, check gitLineStat for unsaved changes
	if e.openFiles[e.activeTab].gitLineStat != nil {
		tabPath := e.openFiles[e.activeTab].filepath
		if tabPath == "" {
			tabPath = e.openFiles[e.activeTab].filename
		}
		absTabPath, err := filepath.Abs(tabPath)
		if err == nil {
			tabRel, err := filepath.Rel(e.git.root, absTabPath)
			if err == nil && filepath.ToSlash(tabRel) == rel {
				for _, st := range e.openFiles[e.activeTab].gitLineStat {
					if st != ' ' {
						return colNumber, true
					}
				}
			}
		}
	}

	switch {
	case s.Worktree == git.Modified || s.Staging == git.Modified:
		return colNumber, true
	case s.Worktree == git.Added || s.Staging == git.Added:
		return colGreen, true
	case s.Worktree == git.Deleted || s.Staging == git.Deleted:
		return colRed, true
	}
	return 0, false
}

func (e *Editor) cmdDiff() {
	if e.filename == "" {
		e.msg("diff: no file")
		return
	}
	tab := e.activeFile()
	if tab.headContent == "" {
		e.msg("diff: no committed version to compare against")
		return
	}
	e.diffHunks = computeDiff(tab.headContent, e.buffer)
	if len(e.diffHunks) == 0 {
		e.msg("diff: no changes")
		return
	}
	e.diffLines = nil
	for _, h := range e.diffHunks {
		e.diffLines = append(e.diffLines, h.lines...)
	}
	e.diffIdx = 0
	e.diffOff = 0
	e.showDiff = true
	e.msg(fmt.Sprintf("diff: %d hunk(s)", len(e.diffHunks)))
}

func (e *Editor) diffRevertHunk() {
	if e.diffIdx < 0 || e.diffIdx >= len(e.diffHunks) {
		return
	}
	hunk := e.diffHunks[e.diffIdx]
	tab := e.activeFile()
	if tab.headContent == "" {
		return
	}

	headLines := strings.Split(tab.headContent, "\n")
	if len(headLines) == 1 && headLines[0] == "" {
		headLines = nil
	}

	oldLines := headLines[hunk.oldStart:hunk.oldEnd]

	e.saveUndoState(opNone)

	before := make([]string, hunk.newStart)
	copy(before, e.buffer[:hunk.newStart])
	after := make([]string, len(e.buffer)-hunk.newEnd)
	copy(after, e.buffer[hunk.newEnd:])

	newBuf := make([]string, 0, len(before)+len(oldLines)+len(after))
	newBuf = append(newBuf, before...)
	newBuf = append(newBuf, oldLines...)
	newBuf = append(newBuf, after...)
	if len(newBuf) == 0 {
		newBuf = []string{""}
	}
	e.buffer = newBuf
	e.cursorInBounds()
	e.setModified()

	e.diffHunks = computeDiff(tab.headContent, e.buffer)
	e.diffLines = nil
	for _, h := range e.diffHunks {
		e.diffLines = append(e.diffLines, h.lines...)
	}
	if e.diffIdx >= len(e.diffHunks) {
		e.diffIdx = len(e.diffHunks) - 1
	}
	if len(e.diffHunks) == 0 {
		e.showDiff = false
		e.msg("diff: all hunks reverted")
	}
}

func (e *Editor) diffUp() {
	if e.diffIdx > 0 {
		e.diffIdx--
	}
	// find first line of the hunk
	for i, dl := range e.diffLines {
		if dl.hunkIdx == e.diffIdx {
			e.diffOff = i
			break
		}
	}
}

func (e *Editor) diffDown() {
	if e.diffIdx < len(e.diffHunks)-1 {
		e.diffIdx++
	}
	// find first line of the hunk
	for i, dl := range e.diffLines {
		if dl.hunkIdx == e.diffIdx {
			e.diffOff = i
			break
		}
	}
}

func (e *Editor) diffPgUp() {
	rows := e.diffRowCount()
	e.diffOff -= rows
	if e.diffOff < 0 {
		e.diffOff = 0
	}
	if len(e.diffLines) > 0 {
		e.diffIdx = e.diffLines[e.diffOff].hunkIdx
	}
}

func (e *Editor) diffPgDn() {
	rows := e.diffRowCount()
	e.diffOff += rows
	last := len(e.diffLines) - 1
	if e.diffOff > last {
		e.diffOff = last
	}
	if len(e.diffLines) > 0 {
		e.diffIdx = e.diffLines[e.diffOff].hunkIdx
	}
}

func (e *Editor) diffHome() {
	e.diffOff = 0
	if len(e.diffLines) > 0 {
		e.diffIdx = e.diffLines[0].hunkIdx
	}
}

func (e *Editor) diffEnd() {
	last := len(e.diffLines) - 1
	if last >= 0 {
		e.diffOff = last
		e.diffIdx = e.diffLines[last].hunkIdx
	}
}

func (e *Editor) diffRowCount() int {
	_, _, _, h := e.editorBox.GetRect()
	boxH := h - 4
	if boxH < 4 {
		boxH = 4
	}
	return boxH
}
