//go:build darwin
// +build darwin

package fsevents

/*
#cgo LDFLAGS: -framework CoreServices
#include <CoreServices/CoreServices.h>
#include <sys/stat.h>

extern void fsevtCallback(FSEventStreamRef p0, uintptr_t info, size_t p1, char** p2, FSEventStreamEventFlags* p3, FSEventStreamEventId* p4);

static FSEventStreamRef EventStreamCreate(FSEventStreamContext * context, uintptr_t info, CFArrayRef paths, CFTimeInterval latency) {
	context->info = (void*) info;
	return FSEventStreamCreate(NULL, (FSEventStreamCallback) fsevtCallback, context, paths,
		kFSEventStreamEventIdSinceNow, latency, kFSEventStreamCreateFlagFileEvents | kFSEventStreamCreateFlagWatchRoot);
}
*/
import "C"
import (
	"errors"
	"log"
	"os"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/fswatch/fswatch/internal"
)

type Interface struct {
	Latency time.Duration

	mu       sync.Mutex
	streamID int

	stream  C.FSEventStreamRef
	rlref   C.CFRunLoopRef
	obsChan chan []internal.Event
}

var (
	mu      sync.Mutex
	actives = make(map[int]*Interface)
)

//export fsevtCallback
func fsevtCallback(stream C.FSEventStreamRef, info uintptr, numEvents C.size_t,
	cpaths **C.char, cflags *C.FSEventStreamEventFlags, cids *C.FSEventStreamEventId) {

	n := int(numEvents)
	events := make([]internal.Event, n)

	paths := unsafe.Slice(cpaths, n)
	flags := unsafe.Slice(cflags, n)

	for i := range events {
		etype := internal.NOTHING
		// Flags that won't be handled
		//   These ones do not have direct bearing on events we care about:
		//     kFSEventStreamEventFlagNone
		//     kFSEventStreamEventFlagMustScanSubDirs
		//     kFSEventStreamEventFlagUserDropped
		//     kFSEventStreamEventFlagKernelDropped
		//     kFSEventStreamEventFlagEventIdsWrapped
		//     kFSEventStreamEventFlagHistoryDone
		//     kFSEventStreamEventFlagItemIsDir
		//     kFSEventStreamEventFlagItemIsFile
		//     kFSEventStreamEventFlagItemIsHardlink
		//     kFSEventStreamEventFlagItemIsLastHardlink
		//     kFSEventStreamEventFlagItemIsSymlink
		//     kFSEventStreamEventFlagOwnEvent
		//   Indicate that a volume was (un)mounted under a watched directory:
		//     kFSEventStreamEventFlagMount
		//     kFSEventStreamEventFlagUnmount
		//   Can't find docs on this one:
		//     kFSEventStreamEventFlagItemCloned

		// Flags that are handled by this code:
		//   OTHER
		//     kFSEventStreamEventFlagItemChangeOwner
		//     kFSEventStreamEventFlagItemFinderInfoMod
		//     kFSEventStreamEventFlagItemInodeMetaMod
		//     kFSEventStreamEventFlagItemXattrMod
		//
		//   MODIFIED
		//     kFSEventStreamEventFlagItemModified
		//
		//   CREATED and/or DELETED
		//     kFSEventStreamEventFlagItemCreated
		//     kFSEventStreamEventFlagItemRenamed
		//     kFSEventStreamEventFlagItemRemoved
		//     **kFSEventStreamEventFlagRootChanged

		if (flags[i] & C.kFSEventStreamEventFlagItemModified) != 0 {
			etype = internal.MODIFIED
		} else if (flags[i] & C.kFSEventStreamEventFlagItemCreated) != 0 {
			etype = internal.CREATED
		} else if (flags[i] & C.kFSEventStreamEventFlagItemRemoved) != 0 {
			etype = internal.DELETED

		} else if (flags[i]&C.kFSEventStreamEventFlagRootChanged) != 0 ||
			(flags[i]&C.kFSEventStreamEventFlagItemRenamed) != 0 {
			// if the watched path itself is moved/removed we get RootChanged
			// if a path is renamed, we get events for both old and new
			// so we have to check if the file exists in this case
			_, err := os.Stat(C.GoString(paths[i]))
			if errors.Is(err, os.ErrNotExist) {
				etype = internal.DELETED
			} else if err == nil && (flags[i]&C.kFSEventStreamEventFlagItemRenamed) != 0 {
				etype = internal.CREATED
			}
		} else if (flags[i] & (C.kFSEventStreamEventFlagItemChangeOwner |
			C.kFSEventStreamEventFlagItemFinderInfoMod |
			C.kFSEventStreamEventFlagItemInodeMetaMod |
			C.kFSEventStreamEventFlagItemXattrMod)) != 0 {
			etype = internal.OTHER
		} else {
			log.Printf("unexp flags=0x%08x", flags[i])
		}

		if etype != internal.NOTHING {
			events[i] = internal.Event{
				Path: C.GoString(paths[i]),
				Type: etype,
			}
		}
	}

	mu.Lock()
	inter, ok := actives[int(info)]
	mu.Unlock()
	if !ok {
		panic("fsevents received event before ready")
	}
	inter.obsChan <- events
}

// createPaths accepts the user defined set of paths and returns FSEvents
// compatible array of paths
func createPaths(paths []string) C.CFArrayRef {
	cPaths := C.CFArrayCreateMutable(C.kCFAllocatorDefault, C.long(len(paths)), &C.kCFTypeArrayCallBacks)
	for _, path := range paths {
		str := makeCFString(path)
		C.CFArrayAppendValue(cPaths, unsafe.Pointer(str))
	}
	return C.CFArrayRef(cPaths)
}

// makeCFString makes an immutable string with CFStringCreateWithCString.
func makeCFString(str string) C.CFStringRef {
	s := C.CString(str)
	defer C.free(unsafe.Pointer(s))
	return C.CFStringCreateWithCString(C.kCFAllocatorDefault, s, C.kCFStringEncodingUTF8)
}

func (x *Interface) start(paths []string) {
	cPaths := createPaths(paths)
	defer C.CFRelease(C.CFTypeRef(cPaths))

	mu.Lock()
	if x.streamID == 0 {
		x.streamID = len(actives) + 1
	}
	actives[x.streamID] = x
	mu.Unlock()

	context := C.FSEventStreamContext{}
	info := C.uintptr_t(x.streamID)
	cfinv := C.CFTimeInterval(float64(x.Latency) / float64(time.Second))

	x.stream = C.EventStreamCreate(&context, info, cPaths, cfinv)
	x.obsChan = make(chan []internal.Event, 8)

	wait := make(chan struct{})

	go func() {
		runtime.LockOSThread()
		x.rlref = C.CFRunLoopGetCurrent()
		C.CFRetain(C.CFTypeRef(x.rlref))

		C.FSEventStreamScheduleWithRunLoop(x.stream, x.rlref, C.kCFRunLoopDefaultMode)
		C.FSEventStreamStart(x.stream)
		close(wait)
		C.CFRunLoopRun()
	}()

	<-wait
}

// request fsevents to stop streaming events
func (x *Interface) stop() {
	C.FSEventStreamFlushSync(x.stream)
	C.FSEventStreamStop(x.stream)
	C.FSEventStreamInvalidate(x.stream)
	C.FSEventStreamRelease(x.stream)

	C.CFRunLoopStop(x.rlref)
	C.CFRelease(C.CFTypeRef(x.rlref))

	close(x.obsChan)
	mu.Lock()
	delete(actives, x.streamID)
	mu.Unlock()
}
