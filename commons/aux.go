package commons

import (
	"errors"
	"log"
	"os"
	"runtime"
)

const (
	NormalExitCode = iota
	GenericErrorExitCode
	SetupErrorExitCode
)

type setupError struct {
	wrappedError error
}

func (e *setupError) Error() string {
	return e.wrappedError.Error()
}

func NewSetupError(err error) error {
	return &setupError{err}
}

func HandleExitCode(err error) {
	var returnCode int
	var setupError *setupError
	switch {
	case err == nil:
		returnCode = NormalExitCode
	case errors.As(err, &setupError):
		returnCode = SetupErrorExitCode
	default:
		returnCode = GenericErrorExitCode
	}
	os.Exit(returnCode)
}

func DieOnError(err error) {
	// Deprecated: DieOnError should not be used in prod!
	log.Println("[WARN] DieOnError should not be used in prod!")
	_, file, line, ok := runtime.Caller(1)
	if ok {
		log.Printf("[WARN] DieOnError called at %s:%d", file, line)
	}
	if err != nil {
		log.Fatal(err)
	}
}
