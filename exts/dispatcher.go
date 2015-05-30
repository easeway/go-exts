package exts

import (
	"encoding/json"
)

type EventHandler func(MessagePipe, string, RawMessage)
type ActionHandler func(MessagePipe, string, RawMessage) (RawMessage, error)

type Dispatcher interface {
	On(event string, handler EventHandler) Dispatcher
	Do(action string, handler ActionHandler) Dispatcher
}

type DispatcherHandlers interface {
	Dispatcher
	EventHandlers(event string) []EventHandler
	ActionHandler(action string) ActionHandler
}

type dispatcherHandlers struct {
	eventHandlers  map[string][]EventHandler
	actionHandlers map[string]ActionHandler
}

func NewDispatcherHandlers() DispatcherHandlers {
	return &dispatcherHandlers{
		eventHandlers:  make(map[string][]EventHandler),
		actionHandlers: make(map[string]ActionHandler),
	}
}

func (d *dispatcherHandlers) On(event string, handler EventHandler) Dispatcher {
	handlers, exists := d.eventHandlers[event]
	if !exists {
		handlers = []EventHandler{handler}
	} else {
		handlers = append(handlers, handler)
	}
	d.eventHandlers[event] = handlers
	return d
}

func (d *dispatcherHandlers) Do(action string, handler ActionHandler) Dispatcher {
	d.actionHandlers[action] = handler
	return d
}

func (d *dispatcherHandlers) EventHandlers(event string) []EventHandler {
	if handlers, exists := d.eventHandlers[event]; exists {
		return handlers
	}
	return nil
}

func (d *dispatcherHandlers) ActionHandler(action string) ActionHandler {
	return d.actionHandlers[action]
}

type DispatchPipeRunner struct {
	pipe     *PipeBase
	handlers DispatcherHandlers
}

func NewDispatchPipeRunnerWithHandlers(source MessagePipe, handlers DispatcherHandlers) *DispatchPipeRunner {
	d := &DispatchPipeRunner{handlers: handlers}
	d.pipe = ConnectPipe(d, source)
	return d
}

func NewDispatchPipeRunner(source MessagePipe) *DispatchPipeRunner {
	return NewDispatchPipeRunnerWithHandlers(source, NewDispatcherHandlers())
}

func (d *DispatchPipeRunner) On(event string, handler EventHandler) Dispatcher {
	d.handlers.On(event, handler)
	return d
}

func (d *DispatchPipeRunner) Do(action string, handler ActionHandler) Dispatcher {
	d.handlers.Do(action, handler)
	return d
}

func (d *DispatchPipeRunner) Close() error {
	return nil
}

func (d *DispatchPipeRunner) Pipe() MessagePipe {
	return d.pipe
}

func (d *DispatchPipeRunner) Recv(p *PipeBase, pkt *RecvPacket) *RecvPacket {
	switch pkt.Message.Event {
	case MsgNotify:
		if handlers := d.handlers.EventHandlers(pkt.Message.Name); handlers != nil {
			for _, handler := range handlers {
				go func(handler EventHandler) {
					handler(d.Pipe(), pkt.Message.Name, RawMessage(pkt.Message.Data))
				}(handler)
			}
			return nil
		}
	case MsgInvoke:
		reply := &Message{
			Event: MsgReply,
			Id:    pkt.Message.Id,
			Name:  pkt.Message.Name,
		}
		if handler := d.handlers.ActionHandler(pkt.Message.Name); handler != nil {
			go func() {
				result, err := handler(d.pipe, pkt.Message.Name, RawMessage(pkt.Message.Data))
				reply.Data = json.RawMessage(result)
				if err != nil {
					reply.Error = err.Error()
				}
				d.pipe.Send(reply)
				// TODO send failure
			}()
		} else {
			go func() {
				reply.Error = "Action " + pkt.Message.Name + " not defined"
				d.pipe.Send(reply)
				// TODO send failure
			}()
		}
		return nil
	}
	return pkt
}

func (d *DispatchPipeRunner) Send(p *PipeBase, pkt *SendPacket) *SendPacket {
	return pkt
}
