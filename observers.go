package fswatch

import (
	"path/filepath"
	"strings"

	"github.com/fswatch/fswatch/internal"
)

// ObserveFunc observes an event ev on the watched path.
// If an error is returned, the observer is not called again.
type ObserveFunc func(path string, ev EventType) error

type fx ObserveFunc

func (x fx) Observe(path string, ev EventType) error {
	return x(path, ev)
}

// AsObserver wraps an ObserveFunc into an Observer
func AsObserver(f ObserveFunc) Observer {
	return fx(f)
}

//////////////

type oa struct {
	obs       Observer
	remap     map[string]string
	relprefix string
	absprefix string
}

func (x *oa) O() internal.ObserveFunc {
	return internal.ObserveFunc(x.All)
}

func (x *oa) All(evts []internal.Event) error {
	for _, e := range evts {
		p := e.Path
		if x.remap != nil {
			if p2, ok := x.remap[p]; ok {
				p = p2
			}
		}
		if x.relprefix != x.absprefix {
			p = filepath.Join(x.relprefix, strings.TrimPrefix(p, x.absprefix))
		}
		err := x.obs.Observe(p, EventType(e.Type))
		if err != nil {
			return err
		}
	}
	return nil
}
