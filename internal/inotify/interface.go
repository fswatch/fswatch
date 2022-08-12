//go:build linux
// +build linux

package inotify

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fswatch/fswatch/internal"
	"golang.org/x/sys/unix"
)

// New returns a new inotify-based filesystem watcher.
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

type Interface struct {
	Latency time.Duration

	mu    sync.Mutex
	fd    int
	names map[int]string
	recur map[int]bool
}

func noop() {}

func (x *Interface) readEvents(r io.Reader) (internal.Event, error) {
	evt := internal.Event{
		Type: internal.OTHER,
	}

	ie := unix.InotifyEvent{}
	err := binary.Read(r, binary.LittleEndian, &ie)
	if err != nil {
		return evt, err
	}

	if n, ok := x.names[int(ie.Wd)]; ok {
		evt.Path = n
	}

	if ie.Len > 0 {
		sname := make([]byte, ie.Len)
		_, err = io.ReadFull(r, sname)
		if err != nil {
			return evt, err
		}
		x := bytes.IndexByte(sname, 0)
		if x >= 0 {
			sname = sname[:x]
		}
		evt.Path += string(sname)
	}

	//if (ie.Mask & unix.IN_MOVE) != 0 { // only recursive
	//   the directory containing a moved file...
	//}

	if (ie.Mask & unix.IN_MOVE_SELF) != 0 {
		// evt.Path is the OLD filename
		evt.Type = internal.DELETED
	}

	if (ie.Mask & unix.IN_MODIFY) != 0 {
		evt.Type = internal.MODIFIED
	}

	if (ie.Mask & unix.IN_CREATE) != 0 { // only recursive
		evt.Type = internal.CREATED

		if x.recur[int(ie.Wd)] && ((ie.Mask & unix.IN_ISDIR) != 0) {
			addMask := uint32(unix.IN_ONLYDIR | unix.IN_MASK_ADD | unix.IN_MODIFY | unix.IN_CREATE |
				unix.IN_DELETE | unix.IN_DELETE_SELF | unix.IN_MOVE | unix.IN_MOVE_SELF | unix.IN_ATTRIB)

			wd, werr := unix.InotifyAddWatch(x.fd, evt.Path, addMask)
			if werr == nil {
				x.names[wd] = evt.Path
				x.recur[wd] = true

				if !strings.HasSuffix(evt.Path, "/") {
					x.names[wd] += "/"
				}
			}
		}
	}

	if (ie.Mask&unix.IN_DELETE) != 0 || // only recursive
		(ie.Mask&unix.IN_DELETE_SELF) != 0 {
		evt.Type = internal.DELETED
	}

	return evt, nil
}

// Files watches a list of files, calling the observer with any events.
func (x *Interface) Files(paths []string, obs internal.ObserveFunc) (cancel func(), err error) {
	x.mu.Lock()

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

	fd, err := unix.InotifyInit()
	if err != nil {
		x.mu.Unlock()
		return func() {}, fmt.Errorf("inotify: init error, %w", err)
	}

	file := os.NewFile(uintptr(fd), "")
	x.fd = fd
	if x.recur == nil {
		x.recur = make(map[int]bool)
	}
	// files only
	addMask := uint32(unix.IN_MASK_ADD | unix.IN_MODIFY | unix.IN_DELETE_SELF | unix.IN_MOVE_SELF | unix.IN_ATTRIB)

	x.names = make(map[int]string, len(p2))
	for _, p := range p2 {
		wd, err := unix.InotifyAddWatch(fd, p, addMask)
		if err != nil {
			file.Close()
			x.mu.Unlock()
			return func() {}, err
		}
		x.names[wd] = p
		x.recur[wd] = false
	}

	go func(f *os.File) {
		for {
			// read evts from rd
			evt, err := x.readEvents(f)
			if err != nil {
				f.Close()
				return
			}
			obs([]internal.Event{evt})
		}
	}(file)

	return func() {
		file.Close()
		x.mu.Unlock()
	}, nil
}

// Recursively watches all files/folders under the given path, calling the observer with any events.
func (x *Interface) Recursively(path string, obs internal.ObserveFunc) (cancel func(), err error) {
	x.mu.Lock()

	// inotify is not recursive, but it can watch folders in bulk
	// so we collect a list of all descendant folder names
	var allpaths []string
	err = filepath.Walk(path, func(subpath string, info os.FileInfo, e error) error {
		if info.IsDir() {
			allpaths = append(allpaths, subpath)
		}
		return e
	})
	if err != nil {
		x.mu.Unlock()
		return noop, err
	}

	fd, err := unix.InotifyInit()
	if err != nil {
		x.mu.Unlock()
		return func() {}, fmt.Errorf("inotify: init error, %w", err)
	}

	file := os.NewFile(uintptr(fd), "")
	x.fd = fd
	if x.recur == nil {
		x.recur = make(map[int]bool)
	}

	addMask := uint32(unix.IN_ONLYDIR | unix.IN_MASK_ADD | unix.IN_MODIFY | unix.IN_CREATE |
		unix.IN_DELETE | unix.IN_DELETE_SELF | unix.IN_MOVE | unix.IN_MOVE_SELF | unix.IN_ATTRIB)

	x.names = make(map[int]string, len(allpaths))
	for _, pname := range allpaths {
		wd, err := unix.InotifyAddWatch(fd, pname, addMask)
		if err != nil {
			file.Close()
			x.mu.Unlock()
			return func() {}, err
		}
		x.names[wd] = pname
		if !strings.HasSuffix(pname, "/") {
			x.names[wd] += "/"
		}
		x.recur[wd] = true
	}

	go func(f *os.File) {
		rd := bufio.NewReader(f)
		for {
			// read evts from rd
			evt, err := x.readEvents(rd)
			if err != nil {
				f.Close()
				return
			}

			obs([]internal.Event{evt})
		}
	}(file)

	return func() {
		file.Close()
		x.mu.Unlock()
	}, nil
}
