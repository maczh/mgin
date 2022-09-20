package utils

import (
	"bytes"
	"github.com/maczh/mgin/logs"
	"os/exec"
	"time"
)

func CmdExec(name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return out.String(), err
}

func CmdRunWithTimeout(cmd *exec.Cmd, timeout time.Duration) (error, bool) {
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	var err error
	select {
	case <-time.After(timeout):
		//timeout
		if err = cmd.Process.Kill(); err != nil {
			logs.Error("failed to kill: {}, error: {}", cmd.Path, err)
		}
		go func() {
			<-done // allow goroutine to exit
		}()
		logs.Info("process:{} killed", cmd.Path)
		return err, true
	case err = <-done:
		return err, false
	}
}
