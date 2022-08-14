package fswatch

import (
	"path/filepath"

	"github.com/fswatch/fswatch/internal"
)

type watcher interface {
	Files(paths []string, obs internal.ObserveFunc) (cancel func(), err error)
	Recursively(path string, obs internal.ObserveFunc) (cancel func(), err error)
}

func wrapFiles(w watcher, paths []string, obs ObserveFunc) (cancel func(), err error) {
	var remap map[string]string
	p2s := make([]string, len(paths))
	for i, p := range paths {
		p2, err := filepath.EvalSymlinks(p)
		if err != nil {
			return func() {}, err
		}
		p2, err = filepath.Abs(p2)
		if err != nil {
			return func() {}, err
		}
		p2s[i] = p2

		if p2 != p {
			if remap == nil {
				remap = make(map[string]string, len(paths))
			}
			remap[p2] = p
		}
	}

	x := &oa{obs: obs, remap: remap}
	return w.Files(p2s, x.O())
}

func wrapRecursively(w watcher, path string, obs ObserveFunc) (cancel func(), err error) {
	p2, err := filepath.EvalSymlinks(path)
	if err != nil {
		return func() {}, err
	}
	p2, err = filepath.Abs(p2)
	if err != nil {
		return func() {}, err
	}

	x := &oa{obs: obs, relprefix: path, absprefix: p2}
	c, e := w.Recursively(p2, x.O())
	if e == internal.ErrNotImplemented {
		return func() {}, ErrRecursiveUnsupported
	}
	return c, e
}

type wrap struct {
	w watcher
}

func (x *wrap) File(path string, obs ObserveFunc) (cancel func(), err error) {
	return wrapFiles(x.w, []string{path}, obs)
}

func (x *wrap) Files(paths []string, obs ObserveFunc) (cancel func(), err error) {
	return wrapFiles(x.w, paths, obs)
}

func (x *wrap) Recursively(path string, obs ObserveFunc) (cancel func(), err error) {
	return wrapRecursively(x.w, path, obs)
}
