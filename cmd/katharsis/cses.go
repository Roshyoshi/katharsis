package main

import (
	"context"
	"errors"
	"sync/atomic"

	"katharsis/internal/catalog"
	"katharsis/internal/content/cses"
)

// joinCSES adds the CSES Problem Set to cats: from its cache at once, or,
// on a first run, once it has loaded in the background, as Codeforces
// does, so launch never waits on cses.fi. It returns passwd's CSES account,
// which works from the moment the catalog is in.
func joinCSES(ctx context.Context, cats *catalog.Union) *csesAccount {
	a := &csesAccount{}
	if c, err := cses.Cached(); err == nil {
		cats.Add(c)
		a.c.Store(c)
		return a
	}
	go func() {
		// Offline, Open still serves the task list the binary carries.
		c, err := cses.Open(ctx)
		if err != nil {
			return
		}
		cats.Add(c)
		a.c.Store(c)
	}()
	return a
}

// csesAccount is the CSES catalog as passwd's account (app.Account).
type csesAccount struct{ c atomic.Pointer[cses.Catalog] }

func (a *csesAccount) Name() string { return "CSES" }

func (a *csesAccount) Account() (nick string, ok bool) {
	if c := a.c.Load(); c != nil {
		return c.Account()
	}
	return "", false
}

func (a *csesAccount) Login(ctx context.Context, nick, pass string) error {
	c := a.c.Load()
	if c == nil {
		return errors.New("CSES is still loading; try again in a moment")
	}
	return c.Login(ctx, nick, pass)
}

func (a *csesAccount) Logout() error {
	if c := a.c.Load(); c != nil {
		return c.Logout()
	}
	return nil
}
