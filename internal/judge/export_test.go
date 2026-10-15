package judge

// WaitPCH waits for the PCH builds this process started in the
// background, so a test can remove the cache they write to.
func WaitPCH() { pchWG.Wait() }
