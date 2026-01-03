package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"

	a "lorde.tech/toys/announcement"
	rl "lorde.tech/toys/rate_limiter"
)

const Missing = 0

func main() {
	log := log.New(
		os.Stdout,
		"[SERVER] ",
		log.LUTC|log.Ldate|log.Ltime|log.Lmsgprefix,
	)
	log.Println("Initializing...")

	// db
	db, err := sql.Open("sqlite3", "./recadinhos.db")
	a.DieOnError(err)
	defer db.Close()
	repo := a.NewAnnouncementRepository(db)
	repo.Init()

	// rate limiter
	apiLimiter, err := rl.NewRateLimiter(10)

	repo.List(1, a.Missing, 1, a.Missing)

	/*recado := a.Announcement{
		ID:        a.Missing,
		Version:   a.AlfaVersion,
		Type:      a.Default,
		Timestamp: time.Now(),
		Tenant:    1,
		Parent:    a.Missing,
		Teacher:   1,
		Class:     a.Missing,
		Message:   "Test announcement!",
		Metadata:  nil,
	}
	 err = repo.Save(&recado)
	a.DieOnError(err)
	log.Printf("New ID: %d\n", recado.ID)
	*/
}
