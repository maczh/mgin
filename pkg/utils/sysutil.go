package utils

import (
	"bytes"
	"errors"
	"os/exec"
	"time"

	"github.com/maczh/mgin/v2/pkg/logs"
)

func CmdExec(name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return out.String(), err
}

// CmdRunWithTimeout waits for a started command up to timeout and kills its
// direct process on expiry. It does not terminate descendant processes.
func CmdRunWithTimeout(cmd *exec.Cmd, timeout time.Duration) (error, bool) {
	if cmd == nil {
		return errors.New("command is nil"), false
	}
	if cmd.Process == nil {
		return errors.New("command has not been started"), false
	}
	waitBound := timeout
	if waitBound <= 0 {
		waitBound = time.Second
	}
	if cmd.WaitDelay == 0 || cmd.WaitDelay > waitBound {
		cmd.WaitDelay = waitBound
	}
	done := make(chan error, 1)
	GoSafe(func() {
		done <- cmd.Wait()
	})

	var err error
	select {
	case <-time.After(timeout):
		//timeout
		killErr := cmd.Process.Kill()
		if killErr != nil {
			logs.Error("failed to kill: {}, error: {}", cmd.Path, killErr)
		}
		logs.Info("process:{} killed", cmd.Path)
		// WaitDelay bounds inherited-pipe draining. Keep waiting for cmd.Wait so
		// the direct child is always reaped before this function returns.
		waitErr := <-done
		if killErr != nil {
			return killErr, true
		}
		if errors.Is(waitErr, exec.ErrWaitDelay) {
			return waitErr, true
		}
		return nil, true
	case err = <-done:
		return err, false
	}
}
