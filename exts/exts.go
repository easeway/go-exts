package exts

import (
	"encoding/json"
	"io"
)

const (
	MsgNotify = "E"
	MsgInvoke = "I"
	MsgReply  = "R"
)

type RawMessage []byte

type Message struct {
	Event string          `json:"event"`
	Id    uint32          `json:"id,omitempty"`
	Name  string          `json:"name"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

type MessagePipe interface {
	io.Closer
	Recv() (*Message, error)
	Send(*Message, ...interface{}) error
}

func RunPipe(pipe MessagePipe) error {
	for {
		if msg, err := pipe.Recv(); err != nil {
			return err
		} else if msg == nil {
			return nil
		}
	}
}
