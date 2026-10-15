//go:build qa

package reader

// Prepare and Render are the reader's two stages, exported for the QA
// render audit only: it renders each cached page as the reader would, and
// a second time with the code blanked, to tell prose from code.

// Prepare is the markdown the reader renders for md: math converted, bare
// fences labelled, tabs in code expanded.
func Prepare(md string) string { return prepare(md, true) }

// Render is the page for prepared markdown at wrap width wrap, as the
// viewport scrolls it, without the reader's header and pager.
func Render(prepared string, wrap int) (string, error) { return render(prepared, wrap) }

// Fences cuts markdown into fenced code blocks and the rest; joining the
// texts gives md back.
func Fences(md string) (code []bool, text []string) {
	for _, s := range splitFences(md) {
		code, text = append(code, s.code), append(text, s.text)
	}
	return code, text
}
