package announcement

import "log"

const Missing = 0

func DieOnError(err error) {
	if err != nil {
		log.Fatal(err.Error())
	}
}
