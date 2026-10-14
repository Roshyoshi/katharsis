package judge

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestRSS(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("no rss on " + runtime.GOOS)
	}
	self, err := rss(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if self < 1<<20 || self > 4<<30 {
		t.Errorf("rss(self) = %s, want a Go test binary's few MB", Bytes(self))
	}

	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if n, err := rss(cmd.Process.Pid); err != nil || n <= 0 {
		t.Errorf("rss(sleep) = %d, %v; want a size", n, err)
	}
	cmd.Process.Kill()
	cmd.Wait()
	if n, err := rss(cmd.Process.Pid); err == nil {
		t.Errorf("rss of a reaped process = %d, want an error", n)
	}
}

func TestWatchMemory(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("no rss on " + runtime.GOOS)
	}
	t.Run("over the limit", func(t *testing.T) {
		cmd := exec.Command("sleep", "5")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		stop := watchMemory(cmd.Process, 1) // any process is over 1 byte
		err := cmd.Wait()
		killedAt := stop()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
			t.Errorf("Wait = %v, want killed by SIGKILL", err)
		}
		if killedAt <= 1 {
			t.Errorf("killedAt = %d, want the size it saw", killedAt)
		}
		if el := time.Since(start); el > time.Second {
			t.Errorf("killed after %v, want at the first poll", el)
		}
	})
	t.Run("under the limit", func(t *testing.T) {
		cmd := exec.Command("sleep", "0.2")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		stop := watchMemory(cmd.Process, 1<<40)
		if err := cmd.Wait(); err != nil {
			t.Errorf("Wait = %v, want a clean exit", err)
		}
		if killedAt := stop(); killedAt != 0 {
			t.Errorf("killedAt = %d, want 0", killedAt)
		}
	})
}
