package cses

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestNetwork opens the real catalog into a temp cache and fetches a
// statement from cses.fi. Opt in with KATHARSIS_NET=1. With
// KATHARSIS_CSES_NICK and KATHARSIS_CSES_PASS set it also logs in and
// downloads one task's tests, the part of the CSES flow no fixture can
// prove; the password stays in memory (noKeyring), never the Keychain.
func TestNetwork(t *testing.T) {
	if os.Getenv("KATHARSIS_NET") != "1" {
		t.Skip("set KATHARSIS_NET=1 to use the network")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	c, err := open(ctx, t.TempDir(), site, pace, noKeyring{})
	if err != nil {
		t.Fatal(err)
	}
	if c.listed.IsZero() || len(c.All()) < 300 {
		t.Fatalf("list not read from cses.fi: %d tasks", len(c.All()))
	}
	a, err := c.Fetch(ctx, "cses:1068")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Samples) != 1 || len(a.Tests) != 0 {
		t.Fatalf("logged out: %+v", a)
	}
	nick, pass := os.Getenv("KATHARSIS_CSES_NICK"), os.Getenv("KATHARSIS_CSES_PASS")
	if nick == "" || pass == "" {
		t.Log("no KATHARSIS_CSES_NICK/PASS: tests download not checked")
		return
	}
	if err := c.Login(ctx, nick, pass); err != nil {
		t.Fatal(err)
	}
	a, err = c.Fetch(ctx, "cses:1068")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Tests) == 0 {
		t.Fatal("no tests downloaded")
	}
	t.Logf("cses:1068: %d tests", len(a.Tests))
}
