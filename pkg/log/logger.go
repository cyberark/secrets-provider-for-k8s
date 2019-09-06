package log

import (
	"fmt"
	"log"
	"os"
)

var InfoLogger = log.New(os.Stdout, "INFO:  ", log.LUTC|log.Ldate|log.Ltime|log.Lshortfile)
var ErrorLogger = log.New(os.Stderr, "ERROR: ", log.LUTC|log.Ldate|log.Ltime|log.Lshortfile)

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
	ErrorLogger.Output(2, fmt.Sprintf(errorMessage, args...))
	return fmt.Errorf(fmt.Sprintf(errorMessage, args...))
}
