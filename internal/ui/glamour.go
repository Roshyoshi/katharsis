package ui

import gansi "charm.land/glamour/v2/ansi"

// StyleConfig is the reader's Glamour style, built from the palette in use
// instead of Glamour's fixed 256-color "dark" one: body text is the
// terminal's own foreground, headings are bold, H1 is a "# " heading
// rather than a colored block, and nothing sets a background. Its layout
// (margins, heading prefixes, list indents) is Glamour's default.
//
// Code is the one place hues mean syntax rather than game state, as they
// do in the player's editor: keywords magenta, types and escapes cyan,
// strings green, numbers yellow, preprocessor lines blue, comments
// italic. Inline code is yellow, without the padded background block.
func StyleConfig() gansi.StyleConfig {
	return gansi.StyleConfig{
		Document: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{BlockPrefix: "\n", BlockSuffix: "\n"},
			Margin:         uintp(2),
		},
		BlockQuote: gansi.StyleBlock{Indent: uintp(1), IndentToken: strp(G.Thin.Left + " ")},
		List:       gansi.StyleList{LevelIndent: 2},
		Heading: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{
			BlockSuffix: "\n", Bold: boolp(true),
		}},
		H1:             heading("# "),
		H2:             heading("## "),
		H3:             heading("### "),
		H4:             heading("#### "),
		H5:             heading("##### "),
		H6:             heading("###### "),
		Strikethrough:  gansi.StylePrimitive{CrossedOut: boolp(true)},
		Emph:           gansi.StylePrimitive{Italic: boolp(true)},
		Strong:         gansi.StylePrimitive{Bold: boolp(true)},
		HorizontalRule: gansi.StylePrimitive{Format: "\n--------\n"},
		Item:           gansi.StylePrimitive{BlockPrefix: "• "},
		Enumeration:    gansi.StylePrimitive{BlockPrefix: ". "},
		Task:           gansi.StyleTask{Ticked: "[" + G.Pass + "] ", Unticked: "[ ] "},
		Link:           gansi.StylePrimitive{Underline: boolp(true)},
		Image:          gansi.StylePrimitive{Underline: boolp(true)},
		ImageText:      gansi.StylePrimitive{Format: "Image: {{.text}} " + G.Arrow},
		Code:           gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: color(yellow)}},
		CodeBlock: gansi.StyleCodeBlock{
			StyleBlock: gansi.StyleBlock{Margin: uintp(2)},
			Chroma:     chroma(),
		},
		DefinitionDescription: gansi.StylePrimitive{BlockPrefix: "\n" + G.Arrow + " "},
	}
}

func heading(prefix string) gansi.StyleBlock {
	return gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Prefix: prefix}}
}

// chroma is the syntax highlighting for code blocks; nil, plain code, under
// Mono.
//
// Chroma takes colors as hex. These are the exact values of ANSI 1 to 6 in
// the xterm 256-color table, so Glamour's terminal256 formatter writes them
// as palette indices 1 to 6, which the terminal's theme paints. Glamour
// registers the style once per process, under one name, so the first
// render fixes it; only ANSI ever registers one.
func chroma() *gansi.Chroma {
	if mono {
		return nil
	}
	c := func(h hue) gansi.StylePrimitive { return gansi.StylePrimitive{Color: strp(chromaHex[h])} }
	return &gansi.Chroma{
		Error:               c(red),
		Comment:             gansi.StylePrimitive{Italic: boolp(true)},
		CommentPreproc:      c(blue),
		Keyword:             c(magenta),
		KeywordReserved:     c(magenta),
		KeywordNamespace:    c(magenta),
		KeywordType:         c(cyan),
		NameBuiltin:         c(cyan),
		LiteralNumber:       c(yellow),
		LiteralString:       c(green),
		LiteralStringEscape: c(cyan),
		GenericDeleted:      c(red),
		GenericInserted:     c(green),
		GenericEmph:         gansi.StylePrimitive{Italic: boolp(true)},
		GenericStrong:       gansi.StylePrimitive{Bold: boolp(true)},
	}
}

var chromaHex = [...]string{
	red: "#800000", green: "#008000", yellow: "#808000",
	blue: "#000080", magenta: "#800080", cyan: "#008080",
}

// color is h as Glamour takes it; nil under Mono.
func color(h hue) *string {
	if mono {
		return nil
	}
	return strp(h.index())
}

func strp(s string) *string { return &s }
func boolp(b bool) *bool    { return &b }
func uintp(u uint) *uint    { return &u }
