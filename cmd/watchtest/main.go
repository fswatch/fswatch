// Command watchtest initiates various filesystem events
// and watches to make sure they are identified correctly.
package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fswatch/fswatch"
)

const EventSentinal = fswatch.EventType(-1)

func randName() string {
	rndbytes := make([]byte, 30)
	rand.Read(rndbytes)
	return base64.URLEncoding.EncodeToString(rndbytes)
}

func printObserver(path string, ev fswatch.EventType) error {
	es := "NOTHING"
	switch ev {
	case fswatch.CREATED:
		es = "CREATED"
	case fswatch.DELETED:
		es = "DELETED"
	case fswatch.MOVED:
		es = "MOVED"
	case fswatch.MODIFIED:
		es = "MODIFIED"
	case fswatch.OTHER:
		es = "OTHER"
	}
	log.Println(path, es)
	return nil
}

func checkFileLoop(fn string) (expect chan fswatch.EventType, obs fswatch.Observer) {
	expect = make(chan fswatch.EventType)
	recvr := make(chan fswatch.EventType)
	obs = fswatch.AsObserver(func(path string, ev fswatch.EventType) error {
		printObserver(path, ev)
		if path != fn {
			log.Fatal("invalid filename for event")
		}
		fmt.Fprintf(os.Stderr, ".")
		os.Stderr.Sync()
		recvr <- ev
		return nil
	})

	go func() {
		for {
			var rc fswatch.EventType
			ev := <-expect
			log.Println("Expect", ev)
			if ev == EventSentinal {
				fmt.Fprintf(os.Stderr, ".\n")
				os.Stderr.Sync()
				log.Println("Done watching file. Tests passed!")
				return
			}
			for rc = range recvr {
				log.Println("Got", rc)
				if ev == rc {
					break
				}
				if rc == fswatch.OTHER {
					continue
				}
			}
			if ev == rc {
				continue
			}
			log.Fatal("invalid event, received", rc, "instead of", ev)
		}
	}()
	return
}

func main() {
	testdur := flag.Duration("d", time.Second*5, "`duration` to run the test for")
	flag.Parse()

	doSingleFileTest(*testdur)
}

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
		cancel1()
		log.Fatal(err)
	}

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
