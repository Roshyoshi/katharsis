package judge_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"katharsis/internal/judge"
)

// orphanEnv names the binary a subprocess of TestOrphanDiesAtCPULimit
// judges; TestMain then runs runOrphanHelper instead of the tests.
const orphanEnv = "KATHARSIS_ORPHAN_HELPER"

// orphanLimits: the judged profile's watchdog kills a run at twice the
// 500 ms budget (1 s of CPU), the wall clock at 1.5 s, and the run's own
// CPU limit is 1 s past the watchdog's.
var orphanLimits = judge.Limits{Time: 500 * time.Millisecond}

const (
	orphanCPUKill = time.Second
	orphanWall    = 1500 * time.Millisecond
	orphanRlimit  = 2 * time.Second
)

// runOrphanHelper is the judge TestOrphanDiesAtCPULimit kills: it judges
// bin, which never ends, and is SIGKILLed meanwhile.
func runOrphanHelper(bin string) int {
	c := judge.Case{In: os.DevNull, Out: os.DevNull}
	r, err := judge.Run(context.Background(), bin, c, orphanLimits, judge.Full)
	fmt.Fprintf(os.Stderr, "the judge outlived its run: %v %q %v\n", r.Verdict, r.Reason, err)
	return 1
}

// A run outlives its judge: kill -9 of the game skips the watchdog and
// every deferred kill. It must not outlive its CPU limit, past which it
// dies of SIGXCPU instead of spinning on, its parent now launchd, until
// reboot.
func TestOrphanDiesAtCPULimit(t *testing.T) {
	needCompiler(t)
	if runtime.GOOS != "darwin" {
		t.Skip("judge.Children lists processes on darwin only")
	}
	t.Parallel()
	b := build(t, "spin.cpp", judge.Full)
	bin, err := filepath.EvalSymlinks(b.Binary) // as ExePath reports it
	if err != nil {
		t.Fatal(err)
	}

	// The judge: this test binary, in runOrphanHelper. TMPDIR is the
	// test's, so the run's working dir its SIGKILL leaves goes with it.
	// -test.run matches nothing, should TestMain not catch orphanEnv: a
	// helper running this test would start one of its own, and so on.
	var stderr bytes.Buffer
	jp := exec.Command(os.Args[0], "-test.run=^$")
	jp.Env = append(os.Environ(), orphanEnv+"="+b.Binary, "TMPDIR="+t.TempDir())
	jp.Stderr = &stderr
	if err := jp.Start(); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	exited := make(chan struct{})
	go func() {
		_ = jp.Wait()
		close(exited)
	}()
	killJudge := func() {
		_ = jp.Process.Kill()
		<-exited
	}

	pid := 0
	for pid == 0 {
		for _, p := range judge.Children(jp.Process.Pid) {
			if judge.ExePath(p) == bin {
				pid = p
			}
		}
		select {
		case <-exited:
			if pid == 0 {
				t.Fatalf("the judge exited without a run: %s", stderr.Bytes())
			}
		case <-time.After(time.Millisecond):
		}
		if pid == 0 && time.Since(started) > time.Minute {
			killJudge()
			t.Fatal("the judge started no run")
		}
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-pid, syscall.SIGKILL) // the run leads its group
		_ = syscall.Kill(pid, syscall.SIGKILL)
	})
	killJudge()

	// Had the judge's watchdog or wall clock fired before it died, the run
	// would be dead or about to be: a machine too busy to let the test
	// kill the judge sooner shows nothing.
	_, cpu, err := judge.Usage(pid)
	if lived := time.Since(started); err != nil || cpu > orphanCPUKill || lived >= orphanWall {
		t.Skipf("starved: the run had %v of CPU (%v) when its judge died after %v", cpu, err, lived)
	}
	for start := time.Now(); ; time.Sleep(20 * time.Millisecond) {
		_, c, err := judge.Usage(pid)
		if err != nil {
			break // gone: launchd reaped it
		}
		cpu = c
		// SIGXCPU comes on a scheduler tick past the limit.
		if cpu > orphanRlimit+500*time.Millisecond {
			t.Fatalf("the run spins on with %v of CPU, its judge dead, past its %v limit", cpu, orphanRlimit)
		}
		// A machine at load 100 gives it a tenth of a core.
		if time.Since(start) > 2*time.Minute {
			t.Skipf("starved: %v of CPU after %v", cpu, time.Since(start))
		}
	}
	t.Logf("the run died at %v of CPU or more, last seen", cpu)
}
