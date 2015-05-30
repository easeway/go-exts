package exts

import (
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrorInvocationTimeout = errors.New("Invocation timeout")
	ErrorInvocationAborted = errors.New("Invocation aborted")
)

type InvokeOptions struct {
	Timeout time.Duration
	Reply   *Message
}

type Invoker interface {
	Invoke(action string, payload RawMessage, timeout time.Duration) (RawMessage, error)
	Notify(event string, payload RawMessage) error
}

type InvokeHelper struct {
	Invoker Invoker
	Action  string
	Payload RawMessage
	Timeout time.Duration
	Reply   RawMessage
	Error   error
}

func InvokeHelp(invoker Invoker) *InvokeHelper {
	return &InvokeHelper{Invoker: invoker}
}

func (v *InvokeHelper) WithAction(action string) *InvokeHelper {
	v.Action = action
	return v
}

func (v *InvokeHelper) WithPayload(payload RawMessage) *InvokeHelper {
	v.Payload = payload
	return v
}

func (v *InvokeHelper) Marshal(payload interface{}) *InvokeHelper {
	if data, err := json.Marshal(payload); err != nil {
		panic(err)
	} else {
		v.Payload = data
	}
	return v
}

func (v *InvokeHelper) WithTimeout(timeout time.Duration) *InvokeHelper {
	v.Timeout = timeout
	return v
}

func (v *InvokeHelper) Invoke() *InvokeHelper {
	v.Reply, v.Error = v.Invoker.Invoke(v.Action, v.Payload, v.Timeout)
	return v
}

func (v *InvokeHelper) Unmarshal(d interface{}) error {
	if v.Error != nil {
		return v.Error
	}
	return json.Unmarshal(v.Reply, d)
}

func (v *InvokeHelper) Result() (RawMessage, error) {
	return v.Reply, v.Error
}

func (v *InvokeHelper) Succeeded() bool {
	return v.Error == nil
}

type NotifyHelper struct {
	Invoker Invoker
	Event   string
	Payload RawMessage
}

func NotifyHelp(invoker Invoker) *NotifyHelper {
	return &NotifyHelper{Invoker: invoker}
}

func (v *NotifyHelper) WithEvent(event string) *NotifyHelper {
	v.Event = event
	return v
}

func (v *NotifyHelper) WithPayload(payload RawMessage) *NotifyHelper {
	v.Payload = payload
	return v
}

func (v *NotifyHelper) Marshal(payload interface{}) *NotifyHelper {
	if data, err := json.Marshal(payload); err != nil {
		panic(err)
	} else {
		v.Payload = data
	}
	return v
}

func (v *NotifyHelper) Notify() error {
	return v.Invoker.Notify(v.Event, v.Payload)
}

type PipeInvoker struct {
	Pipe MessagePipe
}

func (v *PipeInvoker) Invoke(action string, payload RawMessage, timeout time.Duration) (RawMessage, error) {
	option := &InvokeOptions{Timeout: timeout}
	err := v.Pipe.Send(&Message{
		Event: MsgInvoke,
		Name:  action,
		Data:  json.RawMessage(payload),
	}, option)
	if option.Reply == nil {
		return nil, err
	}
	return RawMessage(option.Reply.Data), err
}

func (v *PipeInvoker) Notify(event string, payload RawMessage) error {
	return v.Pipe.Send(&Message{
		Event: MsgNotify,
		Name:  event,
		Data:  json.RawMessage(payload),
	})
}

type InvokerPipe struct {
	Pipe MessagePipe

	invocations map[uint32]chan *Message
	lock        sync.RWMutex
	id          uint32
}

func NewInvokerPipe(source MessagePipe) *InvokerPipe {
	return &InvokerPipe{
		Pipe:        source,
		invocations: make(map[uint32]chan *Message),
	}
}

func (v *InvokerPipe) Recv() (*Message, error) {
	for {
		msg, err := v.Pipe.Recv()
		if err != nil || msg == nil || msg.Event != MsgReply {
			return msg, err
		}
		v.lock.RLock()
		replyCh := v.invocations[msg.Id]
		if replyCh != nil {
			delete(v.invocations, msg.Id)
		}
		v.lock.RUnlock()
		if replyCh != nil {
			replyCh <- msg
		} else {
			return msg, err
		}
	}
}

func (v *InvokerPipe) Send(msg *Message, options ...interface{}) error {
	if msg.Event == MsgInvoke {
		var option *InvokeOptions = nil
		for n, opt := range options {
			if o, ok := opt.(*InvokeOptions); ok {
				option = o
				options = append(options[0:n], options[n+1:]...)
				break
			}
		}

		id := atomic.AddUint32(&v.id, 1)
		msg.Id = id

		reply := make(chan *Message)
		v.lock.Lock()
		v.invocations[id] = reply
		v.lock.Unlock()

		defer func() {
			v.lock.Lock()
			delete(v.invocations, id)
			v.lock.Unlock()
		}()

		if err := v.Pipe.Send(msg, options...); err != nil {
			return err
		}

		var timeCh <-chan time.Time
		if option != nil && option.Timeout > 0 {
			timeCh = time.NewTimer(option.Timeout).C
		} else {
			timeCh = make(chan time.Time)
		}
		select {
		case <-timeCh:
			return ErrorInvocationTimeout
		case msg, ok := (<-reply):
			if !ok {
				return ErrorInvocationAborted
			} else if option != nil {
				option.Reply = msg
			}
		}
		return nil
	}
	return v.Pipe.Send(msg, options...)
}

func (v *InvokerPipe) Close() error {
	v.lock.Lock()
	for _, reply := range v.invocations {
		close(reply)
	}
	v.invocations = make(map[uint32]chan *Message)
	v.lock.Unlock()
	return v.Pipe.Close()
}

func (v *InvokerPipe) Invoke(action string, payload RawMessage, timeout time.Duration) (RawMessage, error) {
	return (&PipeInvoker{v}).Invoke(action, payload, timeout)
}

func (v *InvokerPipe) Notify(event string, payload RawMessage) error {
	return (&PipeInvoker{v}).Notify(event, payload)
}
