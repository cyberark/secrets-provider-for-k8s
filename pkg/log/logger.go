package log

import (
	"fmt"
	"log"
	"os"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log/messages"
)

var stdoutLogger = log.New(os.Stdout, "INFO:  ", log.LUTC|log.Ldate|log.Ltime|log.Lshortfile)
var errorLogger = log.New(os.Stderr, "ERROR: ", log.LUTC|log.Ldate|log.Ltime|log.Lshortfile)
var isDebug = false

/*
	Prints an error message to the error log and returns a new error with the given message.
	This method can receive also more arguments (e.g an external error) and they will be appended to the given error message.

	For example, we have a local method `someMethod()`. This method handles its own error printing and thus we can consume
	the error and not append it to the new error message, as follows:

		returnVal, err := someMethod()
		if err != nil {
			return nil, log.RecordedError("failed to run someMethod")
		}

	On the other hand, if `someMethod()` is a 3rd party method we want to print also the returned error as it wasn't printed
	to the error log. So we'll have the following code:

		returnVal, err := 3rdParty.someMethod()
		if err != nil {
			return nil, log.RecordedError(fmt.Sprintf("failed to run someMethod. Reason: %s", err))
		}
*/
func RecordedError(errorMessage string, args ...interface{}) error {
	errorLogger.Output(2, fmt.Sprintf(errorMessage, args...))
	return fmt.Errorf(fmt.Sprintf(errorMessage, args...))
}

func Error(errorMessage string, args ...interface{}) {
	errorLogger.Output(2, fmt.Sprintf(errorMessage, args...))
}

func Info(infoMessage string, args ...interface{}) {
	stdoutLogger.SetPrefix("INFO: ")
	stdoutLogger.Output(2, fmt.Sprintf(infoMessage, args...))
}

func Warn(infoMessage string, args ...interface{}) {
	stdoutLogger.SetPrefix("WARN: ")
	stdoutLogger.Output(2, fmt.Sprintf(infoMessage, args...))
}

func Debug(infoMessage string, args ...interface{}) {
	if isDebug {
		stdoutLogger.SetPrefix("DEBUG: ")
		stdoutLogger.Output(2, fmt.Sprintf(infoMessage, args...))
	}
}

func EnableDebugMode() {
	stdoutLogger.SetPrefix("DEBUG: ")
	stdoutLogger.Output(2, messages.CSPFK001D)
	isDebug = true
}
