package agenda

import "log"

func DieOnError(err error) {
	if err != nil {
		log.Fatal(err.Error())
	}
}
