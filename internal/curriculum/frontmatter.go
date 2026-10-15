package curriculum

import (
	"errors"
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
	first, rest, _ := strings.Cut(strings.TrimPrefix(src, "\ufeff"), "\n")
	if strings.TrimSuffix(first, "\r") != "---" {
		return "", "", 0, errors.New("no frontmatter")
	}
	for off, line := 0, 2; ; line++ {
		l, _, more := strings.Cut(rest[off:], "\n")
		if strings.TrimRight(l, " \t\r") == "---" {
			return rest[:off], rest[min(off+len(l)+1, len(rest)):], line + 1, nil
		}
		if !more {
			return "", "", 0, errors.New("unterminated frontmatter")
		}
		off += len(l) + 1
	}
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
		key, val, ok := strings.Cut(l, ":")
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		_, dupScalar := f.scalars[key]
		_, dupList := f.lists[key]
		switch {
		case l[0] == ' ' || l[0] == '\t':
			return f, errAt(i, "unexpected indented line %q", t)
		case !ok || key == "":
			return f, errAt(i, "want `key: value`, got %q", t)
		case dupScalar || dupList:
			return f, errAt(i, "duplicate key %q", key)
		}
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
		cont, next := continuation(lines, i, val == "")
		if err := f.set(key, val, cont); err != nil {
			return f, errAt(next-1, "%s: %v", key, err)
		}
		i = next
	}
	return f, nil
}

// continuation collects the lines from i that continue a value: indented
// or blank ones, minus comments and trailing blanks. YAML also lets a block
// list sit at its key's own indentation (Matroid.mdx does), so after an
// empty value an unindented "- item" continues it too. end is the index
// after the last non-blank line taken, so errors name a line of the value.
func continuation(lines []string, i int, empty bool) (cont []string, end int) {
	for end = i; i < len(lines); i++ {
		l := lines[i]
		if more := l == "" || l[0] == ' ' || l[0] == '\t' || empty && isItem(l); !more {
			break
		}
		t := strings.TrimSpace(l)
		if t != "" {
			end = i + 1
		}
		// A blank line inside a folded scalar is a newline; the guide never
		// relies on that, so it joins as a space.
		if !strings.HasPrefix(t, "#") {
			cont = append(cont, t)
		}
	}
	for len(cont) > 0 && cont[len(cont)-1] == "" {
		cont = cont[:len(cont)-1]
	}
	return cont, end
}

// set stores key's value from the rest of its line and its continuation
// lines: a block list, a flow list, or a scalar folded onto one line.
func (f frontmatter) set(key, val string, cont []string) error {
	if val == "" && len(cont) > 0 && isItem(cont[0]) {
		var items []string
		for _, c := range cont {
			switch {
			case c == "":
			case !isItem(c):
				return errors.New("mixed list and scalar")
			default:
				items = append(items, c[1:])
			}
		}
		return f.setList(key, items)
	}
	full := strings.TrimSpace(strings.Join(append([]string{val}, cont...), " "))
	if strings.HasPrefix(full, "[") {
		if !strings.HasSuffix(full, "]") {
			return errors.New("unterminated flow list")
		}
		var items []string
		for _, p := range splitFlow(full[1 : len(full)-1]) {
			if p = strings.TrimSpace(p); p != "" {
				items = append(items, p)
			}
		}
		return f.setList(key, items)
	}
	v, err := yamlScalar(full)
	if err != nil {
		return err
	}
	f.scalars[key] = v
	return nil
}

func (f frontmatter) setList(key string, items []string) error {
	var vals []string
	for _, it := range items {
		v, err := yamlScalar(it)
		if err != nil {
			return err
		}
		vals = append(vals, v)
	}
	f.lists[key] = vals
	return nil
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

// isItem reports whether a trimmed line is a block list item. The dash
// needs a blank after it: "-1 means none" is a scalar.
func isItem(l string) bool { return l == "-" || strings.HasPrefix(l, "- ") }
