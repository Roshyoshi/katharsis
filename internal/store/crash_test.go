package store_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/store"
)

// snapshot is everything a reader can see, for before/after comparisons.
type snapshot struct {
	Meta     game.Meta
	Active   *game.State
	Attempts []store.AttemptRecord
	Boots    []store.BootSummary
	Ratings  []store.RatingPoint
}

func see(t *testing.T, s *store.Store) snapshot {
	t.Helper()
	var (
		v   snapshot
		err error
	)
	if v.Meta, err = s.LoadMeta(); err != nil {
		t.Fatal(err)
	}
	if v.Active, err = s.ActiveBoot(); err != nil {
		t.Fatal(err)
	}
	if v.Attempts, err = s.Attempts(store.AttemptFilter{}); err != nil {
		t.Fatal(err)
	}
	if v.Boots, err = s.Boots(0); err != nil {
		t.Fatal(err)
	}
	if v.Ratings, err = s.RatingHistory(time.Time{}); err != nil {
		t.Fatal(err)
	}
	return v
}

// nextStep is the checkpoint after midBoot: the open door was cleared, the
// rating moved, and a merge window committed bytes. Every table changes.
func nextStep(n int) (game.Meta, *game.State) {
	m := sampleMeta()
	m.Rating += 25
	m.Bank += 120
	st := midBoot(n, t0)
	a := *st.Current
	a.Closed, a.Outcome, a.LastVerdict = a.Opened.Add(20*time.Minute), game.Cleared, "AC"
	st.History = append(st.History, a)
	st.Current = nil
	st.Phase = game.PhaseMerge
	st.Committed, st.Uncommitted = 120, 0
	return m, st
}

var errInjected = errors.New("injected crash")

// TestCheckpointFault fails the transaction after each write in turn and
// checks that readers, in this process and a fresh one, still see exactly
// the previous checkpoint.
func TestCheckpointFault(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"meta", "rating", "boot", "attempts"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			path := tempPath(t)
			s := openClocked(t, path, t0.Add(time.Hour))
			checkpoint(t, s, sampleMeta(), midBoot(1, t0))
			before := see(t, s)

			reached := false
			s.SetFault(func(at string) error {
				if at == stage {
					reached = true
					return errInjected
				}
				return nil
			})
			m, st := nextStep(1)
			if err := s.Checkpoint(m, st); !errors.Is(err, errInjected) {
				t.Fatalf("Checkpoint = %v, want the injected error", err)
			}
			if !reached {
				t.Fatalf("stage %q never reached", stage)
			}
			same(t, "this process", see(t, s), before)
			same(t, "another process", see(t, open(t, path)), before)

			// The aborted transaction left no lock or state behind.
			s.SetFault(nil)
			checkpoint(t, s, m, st)
			if got := see(t, s); len(got.Attempts) != 3 || got.Meta.Bank != m.Bank {
				t.Errorf("after retry: %d attempts, bank %d; want 3, %d",
					len(got.Attempts), got.Meta.Bank, m.Bank)
			}
		})
	}
}

const killEnv = "KATHARSIS_STORE_KILL_DB"

// TestKillMidCheckpoint SIGKILLs a process inside a Checkpoint transaction,
// after SQLite has spilled uncommitted pages into the WAL file, then checks
// that recovery ignores them. This is the physical case the rollback test
// above cannot reach: bytes of the partial write really are on disk.
func TestKillMidCheckpoint(t *testing.T) {
	if path := os.Getenv(killEnv); path != "" {
		killedChild(path)
		return
	}
	t.Parallel()
	path := tempPath(t)
	s := open(t, path)
	checkpoint(t, s, sampleMeta(), midBoot(1, t0))
	before := see(t, s)
	mustClose(t, s)

	cmd := exec.Command(os.Args[0], "-test.run=^TestKillMidCheckpoint$", "-test.count=1")
	cmd.Env = append(os.Environ(), killEnv+"="+path)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("child: %v\n%s", err, out)
	}
	if ws, ok := exit.Sys().(syscall.WaitStatus); !ok || ws.Signal() != syscall.SIGKILL {
		t.Fatalf("child did not die by SIGKILL: %v\n%s", err, out)
	}
	wal, err := os.Stat(path + "-wal")
	if err != nil {
		t.Fatal(err)
	}
	if wal.Size() < 1<<20 {
		t.Fatalf("WAL is %d bytes; the partial write never reached disk, so this proves nothing", wal.Size())
	}
	t.Logf("uncommitted WAL left by the killed writer: %d KiB", wal.Size()>>10)

	s = open(t, path)
	same(t, "after kill", see(t, s), before)
	var ok string
	if err := s.DB().QueryRow(`PRAGMA integrity_check`).Scan(&ok); err != nil || ok != "ok" {
		t.Fatalf("integrity_check = %q, %v", ok, err)
	}
	m, st := nextStep(1)
	checkpoint(t, s, m, st)
}

// killedChild runs in the re-executed test binary. The big log makes the
// boot row larger than SQLite's page cache, forcing it to spill to the WAL
// before the commit that never comes.
func killedChild(path string) {
	s, err := store.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	s.SetFault(func(stage string) error {
		if stage == "attempts" {
			_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
			select {}
		}
		return nil
	})
	m, st := nextStep(1)
	line := strings.Repeat("x", 120)
	for i := range 40000 {
		st.Log = append(st.Log, game.LogLine{At: t0.Add(time.Duration(i) * time.Millisecond), Text: line})
	}
	err = s.Checkpoint(m, st)
	fmt.Fprintln(os.Stderr, "Checkpoint returned instead of being killed:", err)
	os.Exit(3)
}

// TestWALReaders opens the file twice, as `top` in another pane would, and
// checks that a long read neither blocks a checkpoint nor sees half of it.
func TestWALReaders(t *testing.T) {
	t.Parallel()

	t.Run("snapshot isolation", func(t *testing.T) {
		t.Parallel()
		path := tempPath(t)
		w, r := open(t, path), open(t, path)
		m, st := sampleMeta(), midBoot(1, t0)
		checkpoint(t, w, m, st)

		// The store's DSN makes plain BEGIN immediate (a write lock); a
		// read-only transaction begins deferred and pins its snapshot at
		// the first read, as Stats does.
		tx, err := r.DB().BeginTx(t.Context(), &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		bank := func() int64 {
			var raw []byte
			if err := tx.QueryRow(`SELECT json FROM meta`).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var got game.Meta
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			return got.Bank
		}
		if b := bank(); b != m.Bank {
			t.Fatalf("bank = %d before the write", b)
		}

		// Without WAL the write would wait busy_timeout on the open read and
		// fail: the read holds its lock until after the write.
		m.Bank += 100
		checkpoint(t, w, m, st)
		if b := bank(); b != m.Bank-100 {
			t.Errorf("open read saw bank %d mid-transaction, want its snapshot %d", b, m.Bank-100)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		if got, _ := r.LoadMeta(); got.Bank != m.Bank {
			t.Errorf("new read saw bank %d, want %d", got.Bank, m.Bank)
		}
	})

	t.Run("readers during writes", func(t *testing.T) {
		t.Parallel()
		path := tempPath(t)
		w, r := open(t, path), open(t, path)
		m, st := sampleMeta(), midBoot(1, t0)
		m.Bank, st.Uncommitted = 0, 0
		checkpoint(t, w, m, st)

		const writes = 150
		done := make(chan struct{})
		errs := make(chan error, 8)
		for range 4 {
			go func() {
				var lastBank, lastUncommitted int64
				for {
					select {
					case <-done:
						errs <- nil
						return
					default:
					}
					meta, err := r.LoadMeta()
					if err != nil {
						errs <- err
						return
					}
					boot, err := r.ActiveBoot()
					if err != nil || boot == nil {
						errs <- fmt.Errorf("ActiveBoot = %v, %v", boot, err)
						return
					}
					if _, err := r.Stats(); err != nil {
						errs <- err
						return
					}
					// Writes only grow these, so a reader going backwards
					// saw a write that was later undone, or a torn one.
					if meta.Bank < lastBank || boot.Uncommitted < lastUncommitted {
						errs <- fmt.Errorf("went backwards: bank %d -> %d, uncommitted %d -> %d",
							lastBank, meta.Bank, lastUncommitted, boot.Uncommitted)
						return
					}
					lastBank, lastUncommitted = meta.Bank, boot.Uncommitted
				}
			}()
		}
		for i := 1; i <= writes; i++ {
			m.Bank, st.Uncommitted = int64(i), int64(i)
			st.Log = append(st.Log, game.LogLine{At: t0.Add(time.Duration(i) * time.Second), Text: "tick"})
			checkpoint(t, w, m, st)
		}
		close(done)
		for range 4 {
			if err := <-errs; err != nil {
				t.Error(err)
			}
		}
	})
}

// TestForwardCompat edits stored JSON the way a different build would
// leave it: fields this build does not know, or lacks fields it does.
func TestForwardCompat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		state func(map[string]any) // edits the boot snapshot
		meta  func(map[string]any) // edits the Meta row
		want  func(*game.State, *game.Meta)
	}{
		{
			name: "unknown fields are ignored",
			state: func(v map[string]any) {
				v["Elites"] = []any{map[string]any{"Problem": "cf:1A", "Hp": 3}}
				v["Current"].(map[string]any)["Door"].(map[string]any)["Haunted"] = true
				v["Rules"].(map[string]any)["FutureKnob"] = 0.5
			},
			meta: func(v map[string]any) {
				v["Achievements"] = []any{"first-blood"}
				v["Libs"].([]any)[0].(map[string]any)["Tests"] = []any{"1 2"}
			},
			want: func(*game.State, *game.Meta) {},
		},
		{
			name: "missing fields load as zero",
			state: func(v map[string]any) {
				delete(v, "Daemons")
				delete(v["Current"].(map[string]any), "Quacks")
			},
			meta: func(v map[string]any) { delete(v, "Capabilities") },
			want: func(st *game.State, m *game.Meta) {
				st.Daemons, st.Current.Quacks = nil, 0
				m.Capabilities = nil
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := tempPath(t)
			s := open(t, path)
			m, st := sampleMeta(), midBoot(1, t0)
			checkpoint(t, s, m, st)

			edit(t, s, `SELECT state FROM boots WHERE id = 1`, `UPDATE boots SET state = ? WHERE id = 1`, tc.state)
			edit(t, s, `SELECT json FROM meta`, `UPDATE meta SET json = ?`, tc.meta)
			edit(t, s, `SELECT json FROM attempts WHERE boot = 1 AND idx = 0`,
				`UPDATE attempts SET json = ? WHERE boot = 1 AND idx = 0`,
				func(v map[string]any) { v["Replay"] = "base64..." })

			tc.want(st, &m)
			got, err := s.ActiveBoot()
			if err != nil {
				t.Fatal(err)
			}
			same(t, "state", got, st)
			gotMeta, err := s.LoadMeta()
			if err != nil {
				t.Fatal(err)
			}
			same(t, "meta", gotMeta, m)
			recs, err := s.Attempts(store.AttemptFilter{Boot: 1})
			if err != nil {
				t.Fatal(err)
			}
			same(t, "attempt", recs[len(recs)-1].Attempt, st.History[0])
			// And the next checkpoint writes over the edited rows cleanly.
			checkpoint(t, s, m, st)
		})
	}
}

func edit(t *testing.T, s *store.Store, get, put string, f func(map[string]any)) {
	t.Helper()
	var raw []byte
	if err := s.DB().QueryRow(get).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	// UseNumber: a float64 would round the uint64 seed.
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v map[string]any
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	f(v)
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(put, raw); err != nil {
		t.Fatal(err)
	}
}

// BenchmarkCheckpoint is the per-Step cost with a late-boot State: ten
// closed doors and a long dmesg.
func BenchmarkCheckpoint(b *testing.B) {
	s, err := store.Open(filepath.Join(b.TempDir(), "katharsis.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer mustClose(b, s)
	m, st := sampleMeta(), midBoot(1, t0)
	for len(st.History) < 10 {
		st.History = append(st.History, st.History[0])
	}
	for i := range 500 {
		st.Log = append(st.Log, game.LogLine{At: t0.Add(time.Duration(i) * time.Second), Text: "door_4: S2 WA line 2: want 7, got 6"})
	}
	for b.Loop() {
		st.Uncommitted++
		if err := s.Checkpoint(m, st); err != nil {
			b.Fatal(err)
		}
	}
}
