package judge

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"

	"katharsis/internal/units"
)

func TestUsage(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("no usage on " + runtime.GOOS)
	}
	self, _, err := usage(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if self < 1<<20 || self > 4<<30 {
		t.Errorf("usage(self) = %s, want a Go test binary's few MB", units.Mem(self))
	}

	// A shell spinning for 300 ms of CPU must read about that much.
	cmd := exec.Command("sh", "-c", "while :; do :; done")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	var cpu time.Duration
	deadline := time.Now().Add(5 * time.Second)
	for cpu < 300*time.Millisecond && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		n, c, err := usage(cmd.Process.Pid)
		if err != nil || n <= 0 {
			t.Fatalf("usage(spin) = %d, %v, %v; want a size", n, c, err)
		}
		cpu = c
	}
	if cpu < 300*time.Millisecond || cpu > 2*time.Second {
		t.Errorf("usage(spin) cpu = %v after spinning about 300ms", cpu)
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	if n, _, err := usage(cmd.Process.Pid); err == nil {
		t.Errorf("usage of a reaped process = %d, want an error", n)
	}
}

func TestWatch(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("no usage on " + runtime.GOOS)
	}
	killedBySIGKILL := func(t *testing.T, err error) {
		t.Helper()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
			t.Errorf("Wait = %v, want killed by SIGKILL", err)
		}
	}
	t.Run("over the memory limit", func(t *testing.T) {
		cmd := exec.Command("sleep", "5")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		stop := watch(cmd.Process, 1, 0) // any process is over 1 byte
		err := cmd.Wait()
		k := stop()
		killedBySIGKILL(t, err)
		if k.Mem <= 1 || k.CPU {
			t.Errorf("kill = %+v, want the size it saw", k)
		}
		if el := time.Since(start); el > time.Second {
			t.Errorf("killed after %v, want at the first poll", el)
		}
	})
	t.Run("over the CPU limit", func(t *testing.T) {
		cmd := exec.Command("sh", "-c", "while :; do :; done")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		stop := watch(cmd.Process, 1<<40, 200*time.Millisecond)
		err := cmd.Wait()
		k := stop()
		killedBySIGKILL(t, err)
		if !k.CPU || k.Mem != 0 {
			t.Errorf("kill = %+v, want CPU", k)
		}
		if el := time.Since(start); el < 200*time.Millisecond || el > 3*time.Second {
			t.Errorf("killed after %v, want just past 200ms of CPU", el)
		}
	})
	t.Run("idle under the CPU limit", func(t *testing.T) {
		cmd := exec.Command("sleep", "0.5")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		stop := watch(cmd.Process, 1<<40, 100*time.Millisecond)
		if err := cmd.Wait(); err != nil {
			t.Errorf("Wait = %v, want a clean exit: sleeping is not CPU time", err)
		}
		if k := stop(); k != (kill{}) {
			t.Errorf("kill = %+v, want none", k)
		}
	})
}

func TestHeadTail(t *testing.T) {
	h := &headTail{n: 4}
	for _, s := range []string{"ab", "cdef", "ghijklmnop", "qr", "stu"} {
		_, _ = h.Write([]byte(s))
	}
	// head "abcd", tail "rstu", 13 bytes between.
	if got, want := h.String(), "abcd\n… 13B of stderr cut …\nrstu"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
	short := &headTail{n: 4}
	_, _ = short.Write([]byte("abcdefg"))
	if got := short.String(); got != "abcdefg" {
		t.Errorf("String = %q, want everything when nothing was cut", got)
	}
}
