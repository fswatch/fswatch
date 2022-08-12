// Command watchtest initiates various filesystem events
// and watches to make sure they are identified correctly.
package main

import (
	"log"
	"os"
	"time"

	"github.com/fswatch/fswatch"
)

func doTwoFileTest(dur time.Duration) {

	done := make(chan int)
	time.AfterFunc(dur, func() {
		close(done)
	})

	// create 0-byte starter files
	rname1 := randName()
	f, err := os.Create(rname1)
	if err != nil {
		log.Fatal(err)
	}
	f.Close()

	rname2 := randName()
	f, err = os.Create(rname2)
	if err != nil {
		log.Fatal(err)
	}
	f.Close()

	<-time.After(time.Second)

	log.Println("Watching two files: ", rname1, rname2)

	expect, obs := checkFileLoop(rname1, rname2)
	cancel1, err := fswatch.Files([]string{rname1, rname2}, obs)
	if err != nil {
		log.Fatal(err)
	}
	defer cancel1()

	expect <- fswatch.MODIFIED
	f, err = os.OpenFile(rname1, os.O_RDWR, 0666)
	if err != nil {
		log.Fatal(err)
	}
	_, err = f.WriteString("Hello, World!\n")
	if err != nil {
		log.Fatal(err)
	}
	f.Sync()
	f.Close()

	expect <- fswatch.MODIFIED
	f, err = os.OpenFile(rname2, os.O_RDWR, 0666)
	if err != nil {
		log.Fatal(err)
	}
	_, err = f.WriteString("Goodbye, World!\n")
	if err != nil {
		log.Fatal(err)
	}
	f.Sync()
	f.Close()

	expect <- fswatch.DELETED
	err = os.Remove(rname1)
	if err != nil {
		log.Fatal(err)
	}

	rname3 := randName()
	expect <- fswatch.DELETED
	err = os.Rename(rname2, rname3)
	if err != nil {
		log.Fatal(err)
	}

	expect <- EventSentinal
	err = os.Remove(rname3)
	if err != nil {
		log.Fatal(err)
	}

	<-done
	close(expect)
}
