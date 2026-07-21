package main

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/gdamore/tcell/v2"
)

type syntaxToken struct {
	Type  chroma.TokenType
	Value string
	Line  int
	Col   int
}

func langFromExt(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return ""
	}
	switch strings.ToLower(ext) {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".jsx":
		return "jsx"
	case ".rs":
		return "rust"
	case ".c":
		return "c"
	case ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	case ".sh", ".bash":
		return "bash"
	case ".zsh":
		return "bash"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	case ".md", ".markdown":
		return "markdown"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".sql":
		return "sql"
	case ".lua":
		return "lua"
	case ".zig":
		return "zig"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".ex", ".exs":
		return "elixir"
	case ".r", ".R":
		return "r"
	case ".svelte":
		return "svelte"
	case ".vue":
		return "vue"
	case ".proto":
		return "protobuf"
	case ".tf", ".tfvars":
		return "terraform"
	case ".dockerfile":
		return "dockerfile"
	case ".nix":
		return "nix"
	case ".fish":
		return "fish"
	case ".ml", ".mli":
		return "ocaml"
	default:
		return ""
	}
}

func tokenTypeColor(tt chroma.TokenType) tcell.Color {
	if tt >= chroma.Comment && tt < chroma.Generic {
		return colComment
	}
	if tt == chroma.Keyword || tt == chroma.KeywordDeclaration || tt == chroma.KeywordNamespace ||
		tt == chroma.KeywordReserved || tt == chroma.KeywordPseudo || tt == chroma.KeywordConstant ||
		tt == chroma.OperatorWord {
		return colKeyword
	}
	if tt == chroma.KeywordType || tt == chroma.NameClass || tt == chroma.NameDecorator ||
		tt == chroma.NameException {
		return colType
	}
	if tt == chroma.NameFunction || tt == chroma.NameFunctionMagic || tt == chroma.NameProperty {
		return colFunction
	}
	if tt == chroma.NameBuiltin || tt == chroma.NameBuiltinPseudo {
		return colKeyword
	}
	if tt == chroma.NameConstant || tt == chroma.NameVariableGlobal ||
		tt == chroma.NameVariable || tt == chroma.NameNamespace {
		return colType
	}
	if tt >= chroma.LiteralString && tt < chroma.LiteralNumber {
		return colString
	}
	if tt >= chroma.LiteralNumber && tt < chroma.Operator {
		return colNumber
	}
	if tt == chroma.Operator || tt == chroma.Punctuation || tt == chroma.GenericEmph ||
		tt == chroma.GenericStrong || tt == chroma.GenericDeleted || tt == chroma.GenericInserted {
		return colOperator
	}
	if tt == chroma.NameTag {
		return colKeyword
	}
	if tt == chroma.NameAttribute || tt == chroma.NameEntity || tt == chroma.NameLabel ||
		tt == chroma.NameOther || tt == chroma.NameDecorator {
		return colBlue
	}
	return colText
}

func (e *Editor) tokenizeBuffer() {
	tab := e.openFiles[e.activeTab]
	if tab.syntaxTokens != nil {
		return
	}
	lang := langFromExt(tab.filename)
	if lang == "" {
		tab.syntaxTokens = []syntaxToken{}
		return
	}
	lexer := lexers.Get(lang)
	if lexer == nil {
		tab.syntaxTokens = []syntaxToken{}
		return
	}

	text := strings.Join(e.buffer, "\n")

	iterator, err := lexer.Tokenise(nil, text)
	if err != nil {
		tab.syntaxTokens = nil
		return
	}

	all := iterator.Tokens()
	var tokens []syntaxToken
	line := 0
	col := 0
	for _, t := range all {
		if t.Type == chroma.EOFType {
			continue
		}
		skipRecord := t.Type == chroma.Text && strings.TrimSpace(t.Value) == ""
		firstInToken := true
		for _, ch := range t.Value {
			if ch == '\n' {
				line++
				col = 0
				firstInToken = true
			} else {
				if firstInToken && !skipRecord {
					tokens = append(tokens, syntaxToken{
						Type:  t.Type,
						Value: string(ch),
						Line:  line,
						Col:   col,
					})
					firstInToken = false
				}
				col++
			}
		}
	}
	tab.syntaxTokens = tokens
}

func (e *Editor) tokenColorAt(line, col int) tcell.Color {
	tab := e.openFiles[e.activeTab]
	if len(tab.syntaxTokens) == 0 {
		return colText
	}

	toks := tab.syntaxTokens
	lo, hi := 0, len(toks)-1
	var result *syntaxToken
	for lo <= hi {
		mid := lo + (hi-lo)/2
		t := &toks[mid]
		if t.Line < line || (t.Line == line && t.Col <= col) {
			result = t
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if result != nil && result.Line == line {
		return tokenTypeColor(result.Type)
	}
	return colText
}
