package fswatch

import (
	"github.com/fswatch/fswatch/internal/inotify"
)

var impl = inotify.New(nil)

func newImpl(opts map[string]interface{}) watcher {
	return inotify.New(opts)
}
