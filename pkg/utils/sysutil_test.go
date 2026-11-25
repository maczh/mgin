package utils

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestCmdRunWithTimeoutRejectsMissingProcessWithoutPanicking(t *testing.T) {
	for name, cmd := range map[string]*exec.Cmd{
		"nil command": nil,
		"not started": {},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("CmdRunWithTimeout panicked: %v", recovered)
				}
			}()
			if err, timedOut := CmdRunWithTimeout(cmd, 0); err == nil || timedOut {
				t.Fatalf("got (%v, %v), want an input error and no timeout", err, timedOut)
			}
		})
	}
}

func TestCmdRunWithTimeoutKillsStartedProcess(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestCmdRunWithTimeoutHelperProcess$")
	cmd.Env = append(os.Environ(), "MGIN_SYSUTIL_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	err, timedOut := CmdRunWithTimeout(cmd, 20*time.Millisecond)
	if !timedOut {
		t.Fatalf("timedOut=%v, err=%v, want timeout", timedOut, err)
	}
}

func TestCmdRunWithTimeoutHelperProcess(t *testing.T) {
	if os.Getenv("MGIN_SYSUTIL_HELPER") != "1" {
		return
	}
	time.Sleep(5 * time.Second)
}
