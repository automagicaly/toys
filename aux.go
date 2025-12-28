package aux

import "log"

const Missing = 0

func die_on_error(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
