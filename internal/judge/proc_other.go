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

// limitCPU does nothing here: there are no rlimits.
func limitCPU(cmd *exec.Cmd, limit time.Duration) {}
