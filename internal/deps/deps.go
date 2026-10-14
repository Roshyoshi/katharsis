// Package deps pins modules that parallel agents build against, so no two
// agents edit go.mod at once.
//
// SPONGE: delete after integration; real imports keep these in go.mod.
package deps

import (
	_ "charm.land/bubbles/v2/viewport"
	_ "charm.land/glamour/v2"
	_ "github.com/JohannesKaufmann/html-to-markdown/v2"
	_ "golang.org/x/net/html"
	_ "modernc.org/sqlite"
)
