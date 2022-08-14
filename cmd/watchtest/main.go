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
	case fswatch.MODIFIED:
		es = "MODIFIED"
	case fswatch.OTHER:
		es = "OTHER"
		// don't print
		return nil
	}
	log.Println(path, es)
	return nil
}

func checkFileLoop(filenames ...string) (expect chan fswatch.EventType, obs fswatch.ObserveFunc) {
	expect = make(chan fswatch.EventType)
	recvr := make(chan fswatch.EventType)
	obs = fswatch.ObserveFunc(func(path string, ev fswatch.EventType) error {
		printObserver(path, ev)
		valid := false
		for _, fn := range filenames {
			if path == fn {
				valid = true
				break
			}
		}
		if !valid {
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
			//log.Println("Expect", ev)
			if ev == EventSentinal {
				fmt.Fprintf(os.Stderr, ".\n")
				os.Stderr.Sync()
				log.Println("Done watching file. Tests passed!")
				return
			}
			for rc = range recvr {
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
	doTwoFileTest(*testdur)
}
