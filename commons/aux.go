package commons

import (
	"log"
	"runtime"
)

const Missing = 0

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
