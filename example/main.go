package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/fswatch/fswatch"
)

func observer(path string, event fswatch.EventType) error {
	log.Println(path, event)
	return nil
}

func main() {
	flag.Parse()

	tc := time.After(time.Hour)

	var fileBatch []string
	for _, fn := range flag.Args() {
		info, err := os.Stat(fn)
		if err != nil {
			log.Fatal(err)
		}
		if info.IsDir() {
			cancel, err := fswatch.Recursively(fn, fswatch.AsObserver(observer))
			if err != nil {
				log.Fatal(err)
			}
			log.Println("recursively watching: ", fn)
			defer cancel()
			continue
		}
		fileBatch = append(fileBatch, fn)
	}
	if len(fileBatch) > 0 {
		cancel, err := fswatch.Files(fileBatch, fswatch.AsObserver(observer))
		if err != nil {
			log.Fatal(err)
		}
		log.Println("batch watching: ", fileBatch)
		defer cancel()
	}

	<-tc
	return
}
