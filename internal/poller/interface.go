package poller

import (
	"errors"
	"os"
	"sync"
	"time"

	"github.com/fswatch/fswatch/internal"
)

// New returns a new polling filesystem watcher.
// It supports 2 options:
//    "latency" = time.Duration
//    "hash"    = "md5"
//
func New(opts map[string]interface{}) *Interface {
	lat := time.Second / 4
	fast := true
	if opts != nil {
		if x, ok := opts["latency"]; ok {
			lat = x.(time.Duration)
		}
		if x, ok := opts["hash"]; ok && x == "md5" {
			fast = false
		}
	}
	return &Interface{
		Latency:    lat,
		UseHashing: !fast,
	}
}

type Interface struct {
	Latency    time.Duration
	UseHashing bool

	mu    sync.Mutex
	files map[string]*finfo
}

type finfo struct {
	isDir bool
	size  int64
	mtime int64
	perms uint32
	hash  []byte
}

func noop() {}

// Files watches a list of files, calling the observer with any events.
func (x *Interface) Files(paths []string, obs internal.ObserveFunc) (cancel func(), err error) {
	x.mu.Lock()
	if x.Latency <= 0 {
		x.Latency = time.Second / 4
	}

	x.files = make(map[string]*finfo, len(paths))
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return noop, err
		}
		x.files[p] = &finfo{
			isDir: info.IsDir(),
			size:  info.Size(),
			perms: uint32(info.Mode().Perm()),
			mtime: info.ModTime().UnixNano(),
		}
	}

	t := time.NewTicker(x.Latency)

	go func() {
		for range t.C {
			var res []internal.Event

			for p, last := range x.files {
				info, err := os.Stat(p)
				if err == nil {
					x.files[p] = &finfo{
						isDir: info.IsDir(),
						size:  info.Size(),
						perms: uint32(info.Mode().Perm()),
						mtime: info.ModTime().UnixNano(),
					}

					if last.mtime != info.ModTime().UnixNano() ||
						last.size != info.Size() {
						res = append(res, internal.Event{Path: p, Type: internal.MODIFIED})
						continue
					}

					if last.perms != uint32(info.Mode().Perm()) {
						res = append(res, internal.Event{Path: p, Type: internal.OTHER})
						continue
					}
				} else {
					if errors.Is(err, os.ErrNotExist) {
						res = append(res, internal.Event{Path: p, Type: internal.DELETED})
						delete(x.files, p)
						continue
					}
				}
			}

			if len(res) > 0 {
				obs(res)
			}
		}
	}()

	return func() {
		t.Stop()
		x.mu.Unlock()
	}, nil
}

// Files watches a list of files, calling the observer with any events.
func (x *Interface) Recursively(path string, obs internal.ObserveFunc) (cancel func(), err error) {
	return noop, internal.ErrNotImplemented
}
