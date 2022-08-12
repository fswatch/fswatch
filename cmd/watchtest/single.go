// Command watchtest initiates various filesystem events
// and watches to make sure they are identified correctly.
package main

import (
	"log"
	"os"
	"time"

	"github.com/fswatch/fswatch"
)

func doSingleFileTest(dur time.Duration) {

	done := make(chan int)
	time.AfterFunc(dur, func() {
		close(done)
	})

	// create 0-byte starter file
	rname1 := randName()
	f, err := os.Create(rname1)
	if err != nil {
		log.Fatal(err)
	}
	f.Close()

	<-time.After(time.Second)

	log.Println("Watching one file: ", rname1)

	expect, obs := checkFileLoop(rname1)
	cancel1, err := fswatch.File(rname1, obs)
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
	f, err = os.OpenFile(rname1, os.O_RDWR, 0666)
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

	expect <- EventSentinal
	f, err = os.Create(rname1)
	if err != nil {
		log.Fatal(err)
	}
	f.Close()

	err = os.Remove(rname1)
	if err != nil {
		log.Fatal(err)
	}

	<-done
	close(expect)
}
