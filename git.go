package main

import (
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
