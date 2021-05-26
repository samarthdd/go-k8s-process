package rebuildexec

import (
	"bytes"
	"fmt"
	"os/exec"
)

const (
	rcSucess = iota
	rcInvalidCommandLine
	rcDllLoadFailure
	rcConfigLoadFailure
	rcProcessingIssue
)

const (
	rcSucessDesc             = "rcSucessDesc : Test completed successfully"
	rcInvalidCommandLineDesc = "rcInvalidCommandLineDesc : Command line argument is invalid"
	rcDllLoadFailureDesc     = "rcDllLoadFailureDesc :Problem loading the DLL/Shared library"
	rcConfigLoadFailureDesc  = "rcConfigLoadFailureDesc : Problem loading the specified configuration file"
	rcProcessingIssueDesc    = "rcProcessingIssueDesc : Problem processing the specified files"
	unkownExitStatusCode     = "unknown exit status code"
)

func gwCliExec(args string) ([]byte, error) {
	cmd := exec.Command("sh", "-c", args)
	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitStarusDesc := CliExitStatus(exitError.ExitCode())
			return nil, fmt.Errorf(exitStarusDesc)
		}
		return nil, err
	}

	b := out.Bytes()
	return b, nil
}

func CliExitStatus(errCode int) string {
	switch errCode {
	case rcSucess:
		return rcSucessDesc
	case rcInvalidCommandLine:
		return rcInvalidCommandLineDesc
	case rcDllLoadFailure:
		return rcDllLoadFailureDesc
	case rcConfigLoadFailure:
		return rcConfigLoadFailureDesc
	case rcProcessingIssue:
		return rcProcessingIssueDesc
	default:
		return fmt.Sprintf("%s : %v", unkownExitStatusCode, errCode)

	}

}
