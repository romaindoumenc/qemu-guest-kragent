package main

import (
	"bytes"
	"log"
	"os/exec"
	"sync"
)

var processes sync.Map

type ExecPID struct {
	PID int `json:"pid"`
}

func CommandFor(args MessageArgs) ExecPID {
	var cmd *exec.Cmd

	if args.Path != "" {
		cmd = exec.Command(args.Path, args.CmdArgs...)
	} else {
		cmd = exec.Command("/bin/sh", "-c")
	}

	if args.CmdInput != nil {
		cmd.Stdin = bytes.NewReader(args.CmdInput)
	}

	if args.CmdCap {
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	if err := cmd.Start(); err != nil {
		log.Printf("cannot start %s: %s", args.Path, err)
		return ExecPID{-1}
	}

	done := make(chan *exec.Cmd)

	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Println("error waiting for command", err)
		}
		done <- cmd
	}()

	processes.Store(cmd.Process.Pid, done)

	return ExecPID{cmd.Process.Pid}
}

type ExecStatus struct {
	Exited   bool   `json:"exited"`   //    true if process has already terminated.
	ExitCode int    `json:"exitcode"` // process exit code if it was normally terminated.
	OutData  []byte `json:"out-data"` // base64-encoded stdout of the process. This field will only be populated after the process exits.
	ErrData  []byte `json:"err-data"`
}

func ResultsOf(args MessageArgs) ExecStatus {
	done, ok := processes.Load(args.StatusPID)
	if !ok {
		return ExecStatus{Exited: true}
	}

	select {
	case cmd := <-done.(chan *exec.Cmd):
		status := ExecStatus{
			Exited:   true,
			ExitCode: cmd.ProcessState.ExitCode(),
		}
		if cmd.Stderr != nil {
			status.OutData = cmd.Stdout.(*bytes.Buffer).Bytes()
			status.ErrData = cmd.Stderr.(*bytes.Buffer).Bytes()
		}
		return status
	default:
		return ExecStatus{}
	}
}
