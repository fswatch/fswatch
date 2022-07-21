package fswatch

import (
	"github.com/fswatch/fswatch/internal/fsevents"
)

var impl = fsevents.New(nil)

func newImpl(opts map[string]interface{}) watcher {
	return fsevents.New(nil)
}
