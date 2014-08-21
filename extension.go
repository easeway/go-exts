package exts

import (
	"bufio"
	"bytes"
	"container/list"
	"encoding/json"
	"io"
	"os"
	"sync"
)

type EventHandler func(name string, data []byte)
type RequestHandler func(name string, data []byte) (interface{}, error)

type Dispatcher interface {
	On(event string, handler EventHandler) Dispatcher
	Do(action string, handler RequestHandler) Dispatcher
	Notify(event string, data interface{}) error
	Run() error
}

func NewDispatcher() Dispatcher {
	return &dispatcher{
		eventHandlers:  make(map[string]*list.List),
		actionHandlers: make(map[string]*list.List),
		input:          os.Stdin,
		output:         os.Stdout,
	}
}

type dispatcher struct {
	mutex          sync.RWMutex
	eventHandlers  map[string]*list.List
	actionHandlers map[string]RequestHandler
	input          io.Reader
	output         io.Writer
}

func (d *dispatcher) On(event string, handler EventHandler) Dispatcher {
	d.mutex.Lock()
	handlers, exists := d.eventHandlers[event]
	if !exists {
		handlers = list.New()
		d.eventHandlers[event] = handlers
	}
	handlers.PushBack(handler)
	d.mutex.Unlock()
	return d
}

func (d *dispatcher) Do(action string, handler RequestHandler) Dispatcher {
	d.mutex.Lock()
	d.actionHandlers[action] = handler
	d.mutex.Unlock()
	return d
}

func (d *dispatcher) Notify(event string, data interface{}) error {
	encoded, err := encodeMessage(eventEvent, event, 0, data)
	if err == nil {
		_, err = d.output.Write(encoded)
	}
	return err
}

func (d *dispatcher) Run() error {
	reader := bufio.NewReader(d.input)
	for {
		line, err := reader.ReadString("\n")
		var msg message
		if json.Unmarshal(bytes.NewBufferString(line), &msg) == nil {
			switch msg.Event {
			case eventEvent:
				d.dispatchEvent(msg.Name, msg.Data)
			case eventInvoke:
				d.dispatchAction(msg.Name, msg.Data, msg.Id)
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *dispatcher) dispatchEvent(name string, data []byte) {
	d.mutex.RLock()
	if handlers, exists := d.eventHandlers[name]; exists {
		for elem := handlers.Front(); elem != nil; elem = elem.Next() {
			handler := elem.Value.(EventHandler)
			if handler != nil {
				handler(name, data)
			}
		}
	}
	d.mutex.RUnlock()
}

func (d *dispatcher) dispatchAction(name string, data []byte, id uint32) {
	d.mutex.RLock()
	handler, exists := d.actionHandlers[name]
	d.mutex.RUnlock()
	if exists && handler != nil {
		var encoded []byte
		reply, err := handler(name, data)
		if err == nil {
			encoded, err = encodeMessage(eventInvoke, name, id, reply)
		}
		if err != nil {
			encoded, err = encodeMessageErr(eventInvoke, name, id, reply, err)
		}
		if err == nil {
			d.output.Write(encoded)
		}
	}
}
