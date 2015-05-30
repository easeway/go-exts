package exts

import (
	"encoding/json"
	"errors"
	"sync/atomic"
	"time"
)

var (
	ErrorInvocationTimeout = errors.New("Invocation timeout")
	ErrorInvocationAborted = errors.New("Invocation aborted")
)

type InvokeOptions struct {
	Timeout time.Duration
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
	Result  RawMessage
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
	v.Result, v.Error = v.Invoker.Invoke(v.Action, v.Payload, v.Timeout)
	return v
}

func (v *InvokeHelper) Unmarshal(d interface{}) error {
	if v.Error != nil {
		return v.Error
	}
	return json.Unmarshal(v.Result, d)
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
	receipt := v.Pipe.Send(&Message{
		Event: MsgInvoke,
		Name:  action,
		Data:  json.RawMessage(payload),
	}, &InvokeOptions{Timeout: timeout})
	if receipt == nil {
		return nil, nil
	}
	if rawMsg, ok := receipt.Data.(RawMessage); ok {
		return rawMsg, receipt.Error
	}
	return nil, receipt.Error
}

func (v *PipeInvoker) Notify(event string, payload RawMessage) error {
	receipt := v.Pipe.Send(&Message{
		Event: MsgNotify,
		Name:  event,
		Data:  json.RawMessage(payload),
	})
	if receipt != nil {
		return receipt.Error
	}
	return nil
}

type InvokerPipeRunner struct {
	pipe        *PipeBase
	invocations map[uint32]chan *Message
	id          uint32
}

func NewInvokerPipeRunner(source MessagePipe) *InvokerPipeRunner {
	v := &InvokerPipeRunner{
		invocations: make(map[uint32]chan *Message),
	}
	v.pipe = ConnectPipe(v, source)
	return v
}

func (v *InvokerPipeRunner) Recv(p *PipeBase, pkt *RecvPacket) *RecvPacket {
	if pkt.Message.Event == MsgReply {
		if reply, exists := v.invocations[pkt.Message.Id]; exists {
			delete(v.invocations, pkt.Message.Id)
			reply <- pkt.Message
			close(reply)
			return nil
		}
	}
	return pkt
}

func (v *InvokerPipeRunner) Send(p *PipeBase, pkt *SendPacket) *SendPacket {
	if pkt.Message.Event == MsgInvoke {
		id := atomic.AddUint32(&v.id, 1)
		pkt.Message.Id = id
		receipt := p.Source.Send(pkt.Message)
		if receipt.Error != nil {
			pkt.Result <- receipt
			return nil
		}

		reply := make(chan *Message)
		v.invocations[id] = reply

		var timeout time.Duration = 0
		for _, opt := range pkt.Options {
			if o, ok := opt.(*InvokeOptions); ok {
				timeout = o.Timeout
			}
		}

		go func() {
			defer func() {
				p.RunCtrl(func() {
					delete(v.invocations, id)
				})
			}()

			var timeCh <-chan time.Time
			if timeout > 0 {
				timeCh = time.NewTimer(timeout).C
			} else {
				timeCh = make(chan time.Time)
			}
			select {
			case <-timeCh:
				pkt.Result <- &SendReceipt{Error: ErrorInvocationTimeout}
			case msg, ok := (<-reply):
				if !ok {
					pkt.Result <- &SendReceipt{Error: ErrorInvocationAborted}
				} else if msg.Error != "" {
					pkt.Result <- &SendReceipt{
						Error: errors.New(msg.Error),
						Data:  RawMessage(msg.Data),
					}
				} else {
					pkt.Result <- &SendReceipt{
						Data: RawMessage(msg.Data),
					}
				}
			}
		}()
		return nil
	}
	return pkt
}

func (v *InvokerPipeRunner) Close() error {
	for _, reply := range v.invocations {
		close(reply)
	}
	return nil
}

func (v *InvokerPipeRunner) Pipe() MessagePipe {
	return v.pipe
}

func (v *InvokerPipeRunner) Invoke(action string, payload RawMessage, timeout time.Duration) (RawMessage, error) {
	return (&PipeInvoker{v.pipe}).Invoke(action, payload, timeout)
}

func (v *InvokerPipeRunner) Notify(event string, payload RawMessage) error {
	return (&PipeInvoker{v.pipe}).Notify(event, payload)
}
