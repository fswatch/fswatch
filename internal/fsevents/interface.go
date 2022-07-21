//go:build darwin
// +build darwin

package fsevents

import (
	"os"
	"time"

	"github.com/fswatch/fswatch/internal"
)

// New returns a new fsevents-based filesystem watcher.
// It supports 1 option:
//    "latency" = time.Duration
//
func New(opts map[string]interface{}) *Interface {
	lat := time.Second / 4
	if opts != nil {
		if x, ok := opts["latency"]; ok {
			lat = x.(time.Duration)
		}
	}
	return &Interface{
		Latency: lat,
	}
}

func noop() {}

// Files watches a list of files, calling the observer with any events.
func (x *Interface) Files(paths []string, obs internal.ObserveFunc) (cancel func(), err error) {
	x.mu.Lock()
	if x.Latency <= 0 {
		x.Latency = time.Second / 4
	}

	/// force stripping of any directories
	p2 := make([]string, 0, len(paths))
	for _, fn := range paths {
		info, err := os.Stat(fn)
		if err != nil {
			x.mu.Unlock()
			return noop, err
		}
		if info.IsDir() {
			continue
		}
		p2 = append(p2, fn)
	}

	x.start(p2)

	go func() {
		for evts := range x.obsChan {
			obs(evts)
		}
	}()

	return func() {
		x.stop()
		x.mu.Unlock()
	}, nil
}

// Recursively watches all files/folders under the given path, calling the observer with any events.
func (x *Interface) Recursively(path string, obs internal.ObserveFunc) (cancel func(), err error) {
	x.mu.Lock()
	if x.Latency <= 0 {
		x.Latency = time.Second / 4
	}

	x.start([]string{path})

	go func() {
		for evts := range x.obsChan {
			obs(evts)
		}
	}()

	return func() {
		x.stop()
		x.mu.Unlock()
	}, nil
}
