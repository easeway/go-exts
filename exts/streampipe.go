package exts

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

type StreamPipe struct {
	Trace     bool
	TraceName string
	reader    io.ReadCloser
	writer    io.WriteCloser
	recvCh    chan *RecvPacket
	sendCh    chan *sendRequest
}

type sendRequest struct {
	content []byte
	result  chan *SendReceipt
}

type readerCloserWrapper struct {
	reader io.Reader
}

func (r *readerCloserWrapper) Read(data []byte) (int, error) {
	return r.reader.Read(data)
}

func (r *readerCloserWrapper) Close() error {
	return nil
}

type writerCloserWrapper struct {
	writer io.Writer
}

func (r *writerCloserWrapper) Write(data []byte) (int, error) {
	return r.writer.Write(data)
}

func (r *writerCloserWrapper) Close() error {
	return nil
}

func NewStreamPipe(reader io.ReadCloser, writer io.WriteCloser) *StreamPipe {
	return &StreamPipe{
		reader: reader,
		writer: writer,
		recvCh: make(chan *RecvPacket),
		sendCh: make(chan *sendRequest),
	}
}

func NewStreamPipeNoCloser(reader io.Reader, writer io.Writer) *StreamPipe {
	return NewStreamPipe(&readerCloserWrapper{reader}, &writerCloserWrapper{writer})
}

func NewStreamPipeRW(rw io.ReadWriteCloser) *StreamPipe {
	return NewStreamPipe(&readerCloserWrapper{rw}, rw)
}

func NewStreamPipeRWNoCloser(rw io.ReadWriter) *StreamPipe {
	return NewStreamPipe(&readerCloserWrapper{rw}, &writerCloserWrapper{rw})
}

func (p *StreamPipe) RecvChan() <-chan *RecvPacket {
	return p.recvCh
}

func (p *StreamPipe) Run() {
	recvCh := make(chan *RecvPacket)
	defer func() {
		close(p.recvCh)
		p.reader.Close()
		p.writer.Close()
	}()
	trace := p.traceFn()
	go processJsonMsg(p.reader, recvCh, trace)
	for {
		select {
		case pkt := <-recvCh:
			go func(pkt *RecvPacket) {
				defer func() {
					recover()
				}()
				p.recvCh <- pkt
			}(pkt)
		case req, ok := (<-p.sendCh):
			if ok {
				size, err := p.writer.Write(append(req.content, '\n'))
				trace(">>", req.content)
				req.result <- &SendReceipt{err, size}
			} else {
				return
			}
		}
	}
}

func (p *StreamPipe) Send(msg *Message, options ...interface{}) *SendReceipt {
	if content, err := json.Marshal(msg); err != nil {
		return &SendReceipt{Error: err}
	} else {
		result := make(chan *SendReceipt)
		p.sendCh <- &sendRequest{content, result}
		return <-result
	}
}

func (p *StreamPipe) Close() error {
	close(p.sendCh)
	return nil
}

func (p *StreamPipe) traceFn() func(dir string, content []byte) {
	if p.Trace {
		logPrefix := p.TraceName
		if logPrefix == "" {
			logPrefix = fmt.Sprintf("[%d] ", os.Getpid())
		} else {
			logPrefix += " "
		}
		return func(dir string, content []byte) {
			log.Println(logPrefix + dir + " " + string(content))
		}
	} else {
		return func(string, []byte) {
		}
	}
}

func processJsonMsg(in io.Reader, report chan<- *RecvPacket, trace func(string, []byte)) {
	reader := bufio.NewReader(in)
	defer func() {
		recover()
	}()
	for {
		data, err := reader.ReadSlice('\n')
		if data != nil {
			msg := &Message{}
			if json.Unmarshal(data, msg) == nil {
				trace("<<", data)
				report <- &RecvPacket{msg, data, nil}
			} else if err == nil {
				trace("!!", data)
				report <- &RecvPacket{nil, data, nil}
			}
		}
		if err != nil {
			report <- &RecvPacket{nil, nil, err}
			break
		}
	}
}
