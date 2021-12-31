package sharedprocnamespace

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	pauseProcessCmd = "pause"
	psCmd           = "ps -o pid= -o comm="
)

type psCmdResult struct {
	procs map[int]string // Maps PID to its corresponding command string
	err   error
}

// RestartApplication restarts the application that is in the same Pod as
// the Secrets Provider by sending the signal corresponding to 'sigName'
// to all processes in all other containers (except for the "/pause" process
// that was started as part of the Pod "sandbox" during Pod startup).
func RestartApplication(signal syscall.Signal) {
	fmt.Printf("***TEMP*** RestartApplication, restartAppSignal = %d\n", signal)
	if signal == 0 {
		return
	}

	procs, err := getProcsToRestart()
	if err != nil {
		fmt.Printf("Error fetching processes to restart: %v\n", err)
		return
	}

	fmt.Printf("Restarting processes:\n")
	signalName := unix.SignalName(signal)
	for pid, cmd := range procs {
		fmt.Printf("Restarting PID %d (command: '%s') by sending %s...\n",
			pid, cmd, signalName)
		syscall.Kill(pid, signal)
	}
}

func getProcsToRestart() (map[int]string, error) {
	myPID := os.Getpid()

	cmd := execCommand(psCmd)
	r, _ := cmd.StdoutPipe()
	scanner := bufio.NewScanner(r)

	cmdResult := make(chan psCmdResult)

	// Use the scanner to scan the command output line-by-line to collect
	// process info.
	go func() {
		procs := map[int]string{}
		for scanner.Scan() {
			// Get a line of command output
			line := scanner.Text()

			// Split line into PID (first field) and process cmd (the remainder)
			fields := strings.SplitN(strings.TrimSpace(line), " ", 2)
			pidStr := fields[0]
			processCmd := fields[1]
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				cmdResult <- psCmdResult{nil, err}
				return
			}

			// Don't restart this process nor the pause process used by the
			// Pod "sandbox".
			if (pid == myPID) || (processCmd == pauseProcessCmd) {
				continue
			}

			procs[pid] = processCmd
		}
		cmdResult <- psCmdResult{procs, nil}
	}()

	// Run the command
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	// Wait for command output to be processed
	result := <-cmdResult
	return result.procs, result.err
}

func execCommand(cmdStr string) *exec.Cmd {
	fields := strings.Fields(cmdStr)
	cmd := fields[0]
	args := fields[1:]
	return exec.Command(cmd, args...)
}
