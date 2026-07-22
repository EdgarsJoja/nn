package main

import "github.com/gdamore/tcell/v2"

type Theme struct {
	Name, Base, Mantle, Surface0, Surface1, Surface2, Text, Subtext0, Blue, Green, Overlay0 string
	Keyword, String, Comment, TypeName, Number, Operator, Function, Red                      string
}

var themes = []Theme{
	{"Catppuccin Mocha", "#1e1e2e", "#181825", "#313244", "#45475a", "#585b70", "#cdd6f4", "#a6adc8", "#89b4fa", "#a6e3a1", "#6c7086", "#cba6f7", "#a6e3a1", "#6c7086", "#f9e2af", "#fab387", "#89b4fa", "#89b4fa", "#f38ba8"},
	{"Catppuccin Latte", "#eff1f5", "#e6e9ef", "#ccd0da", "#bcc0cc", "#acb0be", "#4c4f69", "#6c6f85", "#1e66f5", "#40a02b", "#9ca0b0", "#8839ef", "#40a02b", "#9ca0b0", "#df8e1d", "#fe640b", "#1e66f5", "#1e66f5", "#d20f39"},
	{"Tokyo Night", "#1a1b26", "#16161e", "#24283b", "#2f3546", "#3b4261", "#c0caf5", "#a9b1d6", "#7aa2f7", "#9ece6a", "#565f89", "#bb9af7", "#9ece6a", "#565f89", "#e0af68", "#ff9e64", "#7aa2f7", "#7aa2f7", "#f7768e"},
	{"Tokyo Night Day", "#e1e2e7", "#d4d5db", "#c4c5cd", "#b4b5be", "#a4a5ae", "#3760bf", "#6172b0", "#2e7de9", "#587539", "#848cb5", "#7c3fbf", "#587539", "#848cb5", "#a0682e", "#d2691b", "#2e7de9", "#2e7de9", "#e64553"},
	{"Dracula", "#282a36", "#21222c", "#343746", "#44475a", "#555879", "#f8f8f2", "#cfcfc2", "#8be9fd", "#50fa7b", "#6272a4", "#bd93f9", "#50fa7b", "#6272a4", "#f1fa8c", "#ffb86c", "#8be9fd", "#8be9fd", "#ff5555"},
	{"One Dark", "#282c34", "#21252b", "#2c313a", "#353b45", "#3e4451", "#abb2bf", "#828997", "#61afef", "#98c379", "#5c6370", "#c678dd", "#98c379", "#5c6370", "#e5c07b", "#d19a66", "#61afef", "#61afef", "#e06c75"},
	{"Ayu Light", "#fafafa", "#f0f0f0", "#e6e6e6", "#d9d9d9", "#cccccc", "#5c6166", "#8a9199", "#39bae6", "#86b300", "#abb0b6", "#a37acc", "#86b300", "#abb0b6", "#f2ae49", "#ed9366", "#39bae6", "#39bae6", "#f07171"},
	{"Gruvbox Dark", "#282828", "#1d2021", "#32302f", "#3c3836", "#504945", "#ebdbb2", "#a89984", "#458588", "#98971a", "#928374", "#d3869b", "#98971a", "#928374", "#d79921", "#d65d0e", "#458588", "#458588", "#fb4934"},
	{"Nord", "#2e3440", "#3b4252", "#434c5e", "#4c566a", "#616e88", "#eceff4", "#d8dee9", "#88c0d0", "#a3be8c", "#4c566a", "#81a1c1", "#a3be8c", "#616e88", "#8fbcbb", "#b48ead", "#81a1c1", "#88c0d0", "#bf616a"},
	{"Monokai", "#272822", "#2d2d2d", "#383830", "#49483e", "#75715e", "#f8f8f2", "#cfcfc2", "#66d9ef", "#a6e22e", "#75715e", "#f92672", "#e6db74", "#75715e", "#a6e22e", "#ae81ff", "#66d9ef", "#66d9ef", "#f92672"},
	{"Solarized Dark", "#002b36", "#073642", "#0a3a46", "#115b6b", "#196c7e", "#93a1a1", "#839496", "#268bd2", "#859900", "#586e75", "#dc322f", "#2aa198", "#586e75", "#b58900", "#d33682", "#268bd2", "#268bd2", "#dc322f"},
	{"Rosé Pine", "#191724", "#1f1d2e", "#26233a", "#2a273f", "#312e44", "#e0def4", "#908caa", "#31748f", "#9ccfd8", "#6e6a86", "#eb6f92", "#9ccfd8", "#6e6a86", "#f6c177", "#ebbcba", "#31748f", "#c4a7e7", "#eb6f92"},
	{"GitHub Dark", "#0d1117", "#161b22", "#21262d", "#30363d", "#484f58", "#c9d1d9", "#8b949e", "#58a6ff", "#3fb950", "#6e7681", "#f78166", "#3fb950", "#6e7681", "#d29922", "#db6d28", "#58a6ff", "#bc8cff", "#ff7b72"},
	{"Everforest Dark", "#2d353b", "#272e33", "#343f44", "#3d484d", "#475258", "#d3c6aa", "#9da9a0", "#7fbbb3", "#a7c080", "#859289", "#e67e80", "#a7c080", "#859289", "#dbbc7f", "#e69875", "#7fbbb3", "#d699b6", "#e67e80"},
	{"Kanagawa", "#1f1f28", "#2a2a37", "#363646", "#434357", "#54546d", "#dcd7ba", "#c8c093", "#7e9cd8", "#98bb6c", "#727169", "#957fb8", "#98bb6c", "#727169", "#e6c384", "#ffa066", "#7e9cd8", "#7e9cd8", "#e46876"},
	{"Night Owl", "#011627", "#01111d", "#0b2942", "#0e293f", "#1d3b53", "#d6deeb", "#5f7e97", "#82aaff", "#c5e478", "#637777", "#c792ea", "#ecc48d", "#637777", "#ffcb8b", "#f78c6c", "#82aaff", "#82aaff", "#ef5350"},
	{"Material Palenight", "#292d3e", "#202331", "#2c3043", "#353a50", "#3e4451", "#a6accd", "#676e95", "#82b1ff", "#c3e88d", "#676e95", "#c792ea", "#c3e88d", "#676e95", "#ffcb6b", "#f78c6c", "#82b1ff", "#82b1ff", "#ff5370"},
}

var (
	colBase, colMantle, colSurface0, colSurface1             tcell.Color
	colSurface2, colText, colSubtext0, colBlue, colGreen     tcell.Color
	colOverlay0, colKeyword, colString, colComment, colType  tcell.Color
	colNumber, colOperator, colFunction, colRed              tcell.Color
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
	colRed = tcell.GetColor(t.Red)
}

func (e *Editor) cycleTheme() {
	e.themeIdx = (e.themeIdx + 1) % len(themes)
	applyTheme(themes[e.themeIdx])
	e.editorBox.SetBackgroundColor(colBase)
	e.sidebar.SetBackgroundColor(colMantle)
	e.statusBox.SetBackgroundColor(colSurface1)
	e.inputField.SetFieldBackgroundColor(colSurface0)
	e.msg("theme: " + themes[e.themeIdx].Name)
	e.saveSettings()
}
