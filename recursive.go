package fswatch

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/fswatch/fswatch/internal"
)

// Recursively watches all files/folders under the given path, calling the observer with any events.
// A recurive watch is the only way to receive CREATED events for new files and folders.
//
// Note: a recursive watch is not always supported by the host operating system, in which case
// ErrRecursiveUnsupported is returned. In this situation, this code will function similarly:
//
//   fileset, _ := fswatch.EnumerateFiles(path, true)
//   cancel, _ := fswatch.Files(fileset, obs)
//
// An important caveat of the code above: you will not receive CREATED notifications for new files.
func Recursively(path string, obs Observer) (cancel func(), err error) {
	p2, err := filepath.EvalSymlinks(path)
	if err != nil {
		return func() {}, err
	}
	p2, err = filepath.Abs(p2)
	if err != nil {
		return func() {}, err
	}

	x := &oa{obs: obs, relprefix: path, absprefix: p2}
	c, e := impl.Recursively(p2, x.O())
	if e == internal.ErrNotImplemented {
		return func() {}, ErrRecursiveUnsupported
	}
	return c, e
}

// ErrRecursiveUnsupported is returned when the host OS does not support a recursive filesystem watch.
// See the documentation for Recursively for potential workarounds.
var ErrRecursiveUnsupported = errors.New("fswatch: recursive watch not supported")

// EnumerateFiles is a helper function to enumerate files for a call to Files, useful when
// Recursively watching is not supported by the host operating system.
func EnumerateFiles(path string, recursive bool) (files []string, err error) {
	var defErr error
	if !recursive {
		defErr = filepath.SkipDir
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return files, filepath.Walk(path, func(pp string, d os.FileInfo, err error) error {
		if d.IsDir() {
			if pp == path {
				return nil
			}
			return defErr
		}

		files = append(files, filepath.Join(path, pp))
		return err
	})
}
