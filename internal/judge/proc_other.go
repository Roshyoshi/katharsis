//go:build !unix

package judge

import (
	"os/exec"
	"time"
)

// inGroup leaves exec's default cancel, which kills the process alone:
// there are no process groups to kill here.
func inGroup(cmd *exec.Cmd) {}

func killGroup(pid int) error { return nil }

// limitCPU and killOrphans do nothing here: there are no rlimits, and
// children finds no processes.
func limitCPU(cmd *exec.Cmd, limit time.Duration) {}

func killOrphans(prefixes []string) {}
