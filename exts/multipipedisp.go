package exts

import (
	"encoding/json"
	"io"
	"sync"
)

type NamedMsgPipe interface {
	MessagePipe
	Name() string
}

type MultiPipeDispatcher interface {
	Dispatcher
	io.Closer
	Run()
	Broadcast(event string, payload RawMessage)
	AddPipe(name string, pipe MessagePipe) MultiPipeDispatcher
}

type namedPipeWrapper struct {
	name string
	pipe MessagePipe
}

func (p *namedPipeWrapper) Close() error {
	return p.pipe.Close()
}

func (p *namedPipeWrapper) RecvChan() <-chan *RecvPacket {
	return p.pipe.RecvChan()
}

func (p *namedPipeWrapper) Send(msg *Message, options ...interface{}) *SendReceipt {
	return p.pipe.Send(msg, options...)
}

func (p *namedPipeWrapper) Run() {
	p.pipe.Run()
}

func (p *namedPipeWrapper) Name() string {
	return p.name
}

type multiPipeDispatcher struct {
	pipes    map[string]MessagePipe
	handlers DispatcherHandlers
}

func NewMultiPipeDispatcher() MultiPipeDispatcher {
	return &multiPipeDispatcher{
		pipes:    make(map[string]MessagePipe),
		handlers: NewDispatcherHandlers(),
	}
}

func (d *multiPipeDispatcher) AddPipe(name string, pipe MessagePipe) MultiPipeDispatcher {
	d.pipes[name] = &namedPipeWrapper{name, NewDispatchPipeRunnerWithHandlers(pipe, d.handlers).Pipe()}
	return d
}

func (d *multiPipeDispatcher) On(event string, handler EventHandler) Dispatcher {
	d.handlers.On(event, handler)
	return d
}

func (d *multiPipeDispatcher) Do(action string, handler ActionHandler) Dispatcher {
	d.handlers.Do(action, handler)
	return d
}

func (d *multiPipeDispatcher) Broadcast(event string, payload RawMessage) {
	msg := &Message{
		Event: MsgNotify,
		Name:  event,
		Data:  json.RawMessage(payload),
	}
	for _, pipe := range d.pipes {
		go pipe.Send(msg)
	}
}

func (d *multiPipeDispatcher) Run() {
	var wg sync.WaitGroup
	for _, pipe := range d.pipes {
		wg.Add(1)
		go func() {
			UnconnectedEnd(pipe)
			pipe.Run()
			wg.Done()
		}()
	}
	wg.Wait()
}

func (d *multiPipeDispatcher) Close() error {
	for _, pipe := range d.pipes {
		go pipe.Close()
	}
	return nil
}
