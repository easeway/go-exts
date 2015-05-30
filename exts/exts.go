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
	Error string          `json:"error"`
}

type RecvPacket struct {
	Message *Message
	Raw     RawMessage
	Error   error
}

type SendPacket struct {
	Message *Message
	Options []interface{}
	Result  chan *SendReceipt
}

type SendReceipt struct {
	Error error
	Data  interface{}
}

type MessagePipe interface {
	io.Closer
	RecvChan() <-chan *RecvPacket
	Send(*Message, ...interface{}) *SendReceipt
	Run()
}
