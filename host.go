package exts

import (
	"container/list"
	"errors"
	"sync"
)

const (
	eventNull  = 0
	eventData  = 1
	eventError = 2
)

type hostEvent struct {
	eventType int
	extEvent  *Event
	extError  error
}

type extsHost struct {
	stopping bool
	mutex    sync.RWMutex
	cond     *sync.Cond
	events   *list.List
	exts     map[string]*extMonitor
	extsWg   sync.WaitGroup
}

var (
	errorHostStopped = errors.New("Extension Host is shutdown")
	errorExtLoaded   = errors.New("Extension already loaded")
)

func NewHost() ExtsHost {
	host := &extsHost{
		stopping: false,
		events:   list.New(),
		exts:     make(map[string]*extMonitor),
	}
	host.cond = sync.NewCond(&host.mutex)
	return host
}

func (host *extsHost) Running() bool {
	return !host.stopping
}

func (host *extsHost) Load(name, path string, args ...string) (Extension, error) {
	host.mutex.Lock()
	defer host.mutex.Unlock()
	if host.stopping {
		return nil, errorHostStopped
	} else if host.exts[name] != nil {
		return nil, errorExtLoaded
	} else {
		monitor := newMonitor(host, name, path, args)
		host.exts[name] = monitor
		host.extsWg.Add(1)
		go monitor.run()
		return monitor, nil
	}
}

func (host *extsHost) Find(name string) Extension {
	host.mutex.RLock()
	defer host.mutex.RUnlock()
	return host.exts[name]
}

func (host *extsHost) WaitEvent() (*Event, error) {
	host.cond.L.Lock()
	defer host.cond.L.Unlock()

	for {
		if host.stopping {
			return nil, errorHostStopped
		}

		elem := host.events.Front()
		if elem == nil {
			host.cond.Wait()
			elem = host.events.Front()
		}

		if elem != nil {
			host.events.Remove(elem)
			event := elem.Value.(*hostEvent)
			switch event.eventType {
			case eventData:
				return event.extEvent, nil
			case eventError:
				return event.extEvent, event.extError
			}
		}
	}

	return nil, nil
}

func (host *extsHost) pushEvent(event *hostEvent) error {
	host.cond.L.Lock()
	defer host.cond.L.Unlock()
	if host.stopping {
		return errorHostStopped
	}
	host.events.PushBack(event)
	host.cond.Signal()
	return nil
}

func (host *extsHost) remove(name string) {
	host.mutex.Lock()
	defer host.mutex.Unlock()
	delete(host.exts, name)
}

func (host *extsHost) Shutdown() {
	host.cond.L.Lock()
	host.stopping = true
	host.cond.Signal()
	host.cond.L.Unlock()
	host.extsWg.Wait()
}
