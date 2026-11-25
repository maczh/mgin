package utils

import (
	"bytes"
	"fmt"
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
	cmd.Env = append(os.Environ(), "MGIN_SYSUTIL_HELPER=parent")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.WaitDelay = time.Hour
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	const timeout = 100 * time.Millisecond
	err, timedOut := CmdRunWithTimeout(cmd, timeout)
	if !timedOut {
		t.Fatalf("timedOut=%v, err=%v, want timeout", timedOut, err)
	}
	if cmd.ProcessState == nil {
		t.Fatal("timed out command was not reaped before return")
	}
	if cmd.WaitDelay > timeout {
		t.Fatalf("WaitDelay=%s exceeds timeout bound %s", cmd.WaitDelay, timeout)
	}
	var childPID int
	if _, scanErr := fmt.Sscanf(stdout.String(), "%d", &childPID); scanErr != nil || childPID <= 0 {
		t.Fatalf("failed to read child PID from helper output %q: %v", stdout.String(), scanErr)
	}
	if child, findErr := os.FindProcess(childPID); findErr == nil {
		_ = child.Kill()
	}
}

func TestCmdRunWithTimeoutHelperProcess(t *testing.T) {
	switch os.Getenv("MGIN_SYSUTIL_HELPER") {
	case "child":
		time.Sleep(5 * time.Second)
	case "parent":
		child := exec.Command(os.Args[0], "-test.run=^TestCmdRunWithTimeoutHelperProcess$")
		child.Env = append(os.Environ(), "MGIN_SYSUTIL_HELPER=child")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			fmt.Fprintln(os.Stdout, "failed to start child:", err)
			return
		}
		fmt.Fprintln(os.Stdout, child.Process.Pid)
		time.Sleep(5 * time.Second)
	default:
		return
	}
}
