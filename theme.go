package main

import "github.com/gdamore/tcell/v2"

type Theme struct {
	Name, Base, Mantle, Surface0, Surface1, Surface2, Text, Subtext0, Blue, Green, Overlay0 string
	Keyword, String, Comment, TypeName, Number, Operator, Function                           string
}

var themes = []Theme{
	{"Catppuccin Mocha", "#1e1e2e", "#181825", "#313244", "#45475a", "#585b70", "#cdd6f4", "#a6adc8", "#89b4fa", "#a6e3a1", "#6c7086", "#cba6f7", "#a6e3a1", "#6c7086", "#f9e2af", "#fab387", "#89b4fa", "#89b4fa"},
	{"Catppuccin Latte", "#eff1f5", "#e6e9ef", "#ccd0da", "#bcc0cc", "#acb0be", "#4c4f69", "#6c6f85", "#1e66f5", "#40a02b", "#9ca0b0", "#8839ef", "#40a02b", "#9ca0b0", "#df8e1d", "#fe640b", "#1e66f5", "#1e66f5"},
	{"Tokyo Night", "#1a1b26", "#16161e", "#24283b", "#2f3546", "#3b4261", "#c0caf5", "#a9b1d6", "#7aa2f7", "#9ece6a", "#565f89", "#bb9af7", "#9ece6a", "#565f89", "#e0af68", "#ff9e64", "#7aa2f7", "#7aa2f7"},
	{"Tokyo Night Day", "#e1e2e7", "#d4d5db", "#c4c5cd", "#b4b5be", "#a4a5ae", "#3760bf", "#6172b0", "#2e7de9", "#587539", "#848cb5", "#7c3fbf", "#587539", "#848cb5", "#a0682e", "#d2691b", "#2e7de9", "#2e7de9"},
	{"Dracula", "#282a36", "#21222c", "#343746", "#44475a", "#555879", "#f8f8f2", "#cfcfc2", "#8be9fd", "#50fa7b", "#6272a4", "#bd93f9", "#50fa7b", "#6272a4", "#f1fa8c", "#ffb86c", "#8be9fd", "#8be9fd"},
	{"One Dark", "#282c34", "#21252b", "#2c313a", "#353b45", "#3e4451", "#abb2bf", "#828997", "#61afef", "#98c379", "#5c6370", "#c678dd", "#98c379", "#5c6370", "#e5c07b", "#d19a66", "#61afef", "#61afef"},
	{"Ayu Light", "#fafafa", "#f0f0f0", "#e6e6e6", "#d9d9d9", "#cccccc", "#5c6166", "#8a9199", "#39bae6", "#86b300", "#abb0b6", "#a37acc", "#86b300", "#abb0b6", "#f2ae49", "#ed9366", "#39bae6", "#39bae6"},
	{"Gruvbox Dark", "#282828", "#1d2021", "#32302f", "#3c3836", "#504945", "#ebdbb2", "#a89984", "#458588", "#98971a", "#928374", "#d3869b", "#98971a", "#928374", "#d79921", "#d65d0e", "#458588", "#458588"},
}

var (
	colBase, colMantle, colSurface0, colSurface1             tcell.Color
	colSurface2, colText, colSubtext0, colBlue, colGreen     tcell.Color
	colOverlay0, colKeyword, colString, colComment, colType  tcell.Color
	colNumber, colOperator, colFunction                      tcell.Color
)

func applyTheme(t Theme) {
	colBase = tcell.GetColor(t.Base)
	colMantle = tcell.GetColor(t.Mantle)
	colSurface0 = tcell.GetColor(t.Surface0)
	colSurface1 = tcell.GetColor(t.Surface1)
	colSurface2 = tcell.GetColor(t.Surface2)
	colText = tcell.GetColor(t.Text)
	colSubtext0 = tcell.GetColor(t.Subtext0)
	colBlue = tcell.GetColor(t.Blue)
	colGreen = tcell.GetColor(t.Green)
	colOverlay0 = tcell.GetColor(t.Overlay0)
	colKeyword = tcell.GetColor(t.Keyword)
	colString = tcell.GetColor(t.String)
	colComment = tcell.GetColor(t.Comment)
	colType = tcell.GetColor(t.TypeName)
	colNumber = tcell.GetColor(t.Number)
	colOperator = tcell.GetColor(t.Operator)
	colFunction = tcell.GetColor(t.Function)
}

func (e *Editor) cycleTheme() {
	e.themeIdx = (e.themeIdx + 1) % len(themes)
	applyTheme(themes[e.themeIdx])
	e.editorBox.SetBackgroundColor(colBase)
	e.sidebar.SetBackgroundColor(colMantle)
	e.statusBox.SetBackgroundColor(colSurface1)
	e.inputField.SetFieldBackgroundColor(colSurface0)
	e.msg("theme: " + themes[e.themeIdx].Name)
}
