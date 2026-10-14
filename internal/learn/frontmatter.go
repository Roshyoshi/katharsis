package learn

import (
	"fmt"
	"strconv"
	"strings"
)

// The guide's frontmatter is a small YAML subset: top-level `key: value`
// pairs whose values are plain, single- or double-quoted scalars (possibly
// folded over indented continuation lines), `- item` block lists, or `[a, b]`
// flow lists. Pulling in a YAML library for that is not worth a new
// dependency, so this parser accepts exactly that subset and rejects the
// rest with a line number.

type frontmatter struct {
	scalars map[string]string
	lists   map[string][]string
	keys    []string
}

func (f frontmatter) str(k string) string { return f.scalars[k] }

// list returns a list value, or a scalar split on commas (the guide writes
// both `author: A, B` and block lists).
func (f frontmatter) list(k string) []string {
	if l, ok := f.lists[k]; ok {
		return l
	}
	s := f.scalars[k]
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitFrontmatter separates a leading `---` block from the body. It returns
// the frontmatter source, the body, and the 1-based line the body starts on.
func splitFrontmatter(src string) (fm, body string, bodyLine int, err error) {
	src = strings.TrimPrefix(src, "\ufeff")
	if !strings.HasPrefix(src, "---\n") && !strings.HasPrefix(src, "---\r\n") {
		return "", src, 1, fmt.Errorf("no frontmatter")
	}
	rest := src[strings.IndexByte(src, '\n')+1:]
	line := 2
	for off := 0; off <= len(rest); {
		end := strings.IndexByte(rest[off:], '\n')
		var l string
		if end < 0 {
			l = rest[off:]
		} else {
			l = rest[off : off+end]
		}
		if strings.TrimRight(l, " \t\r") == "---" {
			fm = rest[:off]
			if end < 0 {
				return fm, "", line + 1, nil
			}
			return fm, rest[off+end+1:], line + 1, nil
		}
		if end < 0 {
			break
		}
		off += end + 1
		line++
	}
	return "", "", 0, fmt.Errorf("unterminated frontmatter")
}

func parseFrontmatter(src string) (frontmatter, error) {
	f := frontmatter{scalars: map[string]string{}, lists: map[string][]string{}}
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	errAt := func(i int, format string, args ...any) error {
		// +2: line 1 is the opening ---.
		return fmt.Errorf("frontmatter line %d: %s", i+2, fmt.Sprintf(format, args...))
	}
	for i := 0; i < len(lines); {
		l := lines[i]
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			i++
			continue
		}
		if l[0] == ' ' || l[0] == '\t' {
			return f, errAt(i, "unexpected indented line %q", t)
		}
		colon := strings.IndexByte(l, ':')
		if colon <= 0 {
			return f, errAt(i, "want `key: value`, got %q", t)
		}
		key := strings.TrimSpace(l[:colon])
		if _, dup := f.scalars[key]; dup {
			return f, errAt(i, "duplicate key %q", key)
		}
		if _, dup := f.lists[key]; dup {
			return f, errAt(i, "duplicate key %q", key)
		}
		f.keys = append(f.keys, key)
		val := strings.TrimSpace(l[colon+1:])
		i++
		// A quoted scalar may continue on lines at any indentation
		// (js-yaml accepts it, and Complete_Rec.mdx relies on it).
		for val != "" && (val[0] == '\'' || val[0] == '"') && !quotedClosed(val) {
			if i >= len(lines) {
				return f, errAt(i-1, "%s: unterminated quoted scalar", key)
			}
			val += " " + strings.TrimSpace(lines[i])
			i++
		}
		// Collect indented continuation lines. YAML also lets a block list
		// sit at its key's own indentation (Matroid.mdx does).
		var cont []string
		isItem := func(l string) bool { return l == "-" || strings.HasPrefix(l, "- ") }
		for i < len(lines) && (lines[i] == "" || lines[i][0] == ' ' || lines[i][0] == '\t' ||
			val == "" && isItem(lines[i])) {
			if strings.TrimSpace(lines[i]) == "" {
				// A blank line inside a folded scalar is a newline; the
				// guide never relies on that, so treat it as a space.
				cont = append(cont, "")
			} else {
				cont = append(cont, strings.TrimSpace(lines[i]))
			}
			i++
		}
		for len(cont) > 0 && cont[len(cont)-1] == "" {
			cont = cont[:len(cont)-1]
		}
		if val == "" && len(cont) > 0 && strings.HasPrefix(cont[0], "-") {
			var items []string
			for _, c := range cont {
				if c == "" {
					continue
				}
				if !strings.HasPrefix(c, "-") {
					return f, errAt(i-1, "mixed list and scalar under %q", key)
				}
				v, err := yamlScalar(strings.TrimSpace(c[1:]))
				if err != nil {
					return f, errAt(i-1, "%s: %v", key, err)
				}
				items = append(items, v)
			}
			f.lists[key] = items
			continue
		}
		full := val
		for _, c := range cont {
			if full == "" {
				full = c
			} else {
				full += " " + c
			}
		}
		if strings.HasPrefix(full, "[") {
			if !strings.HasSuffix(full, "]") {
				return f, errAt(i-1, "unterminated flow list under %q", key)
			}
			var items []string
			for _, p := range splitFlow(full[1 : len(full)-1]) {
				if p = strings.TrimSpace(p); p == "" {
					continue
				}
				v, err := yamlScalar(p)
				if err != nil {
					return f, errAt(i-1, "%s: %v", key, err)
				}
				items = append(items, v)
			}
			f.lists[key] = items
			continue
		}
		v, err := yamlScalar(full)
		if err != nil {
			return f, errAt(i-1, "%s: %v", key, err)
		}
		f.scalars[key] = v
	}
	return f, nil
}

// quotedClosed reports whether s, starting with a quote, also ends that
// quoted scalar.
func quotedClosed(s string) bool {
	q := s[0]
	for i := 1; i < len(s); i++ {
		switch {
		case q == '"' && s[i] == '\\':
			i++
		case s[i] == q && q == '\'' && i+1 < len(s) && s[i+1] == '\'':
			i++ // '' is an escaped quote
		case s[i] == q:
			return true
		}
	}
	return false
}

// splitFlow splits a flow list body on commas outside quotes.
func splitFlow(s string) []string {
	var out []string
	var q byte
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case q != 0 && c == q:
			q = 0
		case q == 0 && (c == '\'' || c == '"'):
			q = c
		case q == 0 && c == ',':
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func yamlScalar(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", nil
	case s[0] == '\'':
		if len(s) < 2 || s[len(s)-1] != '\'' {
			return "", fmt.Errorf("unterminated single-quoted scalar %q", s)
		}
		inner := s[1 : len(s)-1]
		if strings.Contains(strings.ReplaceAll(inner, "''", ""), "'") {
			return "", fmt.Errorf("stray quote in %q", s)
		}
		return strings.ReplaceAll(inner, "''", "'"), nil
	case s[0] == '"':
		if len(s) < 2 || s[len(s)-1] != '"' {
			return "", fmt.Errorf("unterminated double-quoted scalar %q", s)
		}
		v, err := strconv.Unquote(s)
		if err != nil {
			return "", fmt.Errorf("bad double-quoted scalar %q", s)
		}
		return v, nil
	}
	// Plain scalar: drop a trailing comment.
	if i := strings.Index(s, " #"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s, nil
}
