// this package really only exists to stop import cycles

package internal

import "errors"

// EventType is the type of events generated by a Watcher.
type EventType int

const (
	NOTHING  EventType = iota // nothing happened
	CREATED                   // something was created
	DELETED                   // something was deleted
	MODIFIED                  // contents were modified
	OTHER                     // something else (metadata?) was modified
)

type Event struct {
	Path string
	Type EventType
}

type ObserveFunc func(evts []Event) error

var ErrNotImplemented = errors.New("not implemented")
