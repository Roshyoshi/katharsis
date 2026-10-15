//go:build qa

package codeforces

import (
	"errors"

	"katharsis/internal/catalog"
)

// Markdown renders id's statement.md and editorial.md ("" without one) from
// the cached index alone, as Fetch would write them, without samples or
// tests. Only the QA harness builds it: the render audit reads all ten
// thousand statements this way, where a Fetch per problem would download
// their tests.
func (c *Catalog) Markdown(id catalog.ID) (statement, editorial string, err error) {
	e, ok := c.entry(id)
	if !ok {
		return "", "", errors.New(string(id) + ": not a Codeforces problem in the catalog")
	}
	t, err := c.readText(e)
	if err != nil {
		return "", "", err
	}
	statement = statementMarkdown(e, t)
	if e.HasEditorial {
		editorial = editorialMarkdown(e, t)
	}
	return statement, editorial, nil
}
