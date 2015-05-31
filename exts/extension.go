package exts

import (
	"log"
	"os"
	"time"
)

type Extension struct {
	Invoker    Invoker
	Dispatcher Dispatcher
	Pipe       MessagePipe
	StreamPipe *StreamPipe
}

func NewExtension() *Extension {
	ext := &Extension{}
	ext.StreamPipe = NewStreamPipeNoCloser(os.Stdin, os.Stdout)
	v := NewInvokerPipe(ext.StreamPipe)
	ext.Invoker = v
	d := NewDispatchPipe(v)
	ext.Dispatcher = d
	ext.Pipe = d
	return ext
}

func (ext *Extension) Close() error {
	return ext.Pipe.Close()
}

func (ext *Extension) Recv() (*Message, error) {
	return ext.Pipe.Recv()
}

func (ext *Extension) Send(msg *Message, options ...interface{}) error {
	return ext.Pipe.Send(msg, options...)
}

func (ext *Extension) Invoke(action string, payload RawMessage, timeout time.Duration) (RawMessage, error) {
	return ext.Invoker.Invoke(action, payload, timeout)
}

func (ext *Extension) Notify(event string, payload RawMessage) error {
	return ext.Invoker.Notify(event, payload)
}

func (ext *Extension) On(event string, handler EventHandler) Dispatcher {
	ext.Dispatcher.On(event, handler)
	return ext
}

func (ext *Extension) Do(action string, handler ActionHandler) Dispatcher {
	ext.Dispatcher.Do(action, handler)
	return ext
}

func (ext *Extension) InvokeHelp() *InvokeHelper {
	return InvokeHelp(ext.Invoker)
}

func (ext *Extension) NotifyHelp() *NotifyHelper {
	return NotifyHelp(ext.Invoker)
}

func (ext *Extension) Run() error {
	return RunPipe(ext)
}

func (ext *Extension) Logger(prefix string, flag int) *log.Logger {
	return log.New(os.Stderr, prefix, flag)
}
