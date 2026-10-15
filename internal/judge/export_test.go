package judge

// WaitPCH waits for the PCH builds this process started in the
// background, so a test can remove the cache they write to.
func WaitPCH() { pchWG.Wait() }

// Children, ExePath and Usage let a test find a run under a judge it
// started, and watch it once it has killed that judge.
var (
	Children = children
	ExePath  = exePath
	Usage    = usage
)
