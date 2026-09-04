package main

import (
	"os/exec"
	"testing"
	"time"
)

func TestProcessJobTerminatesChildOnClose(t *testing.T) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", "Start-Sleep -Seconds 30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	job, err := createProcessJob(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { finished <- cmd.Wait() }()
	job.Close()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("child process survived closing the Job Object")
	}
}
