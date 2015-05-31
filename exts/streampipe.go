package exts

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

type StreamPipe struct {
	reader io.ReadCloser
	writer io.WriteCloser

	lineReader *bufio.Reader
	err        error
	end        bool
	writeLock  sync.Mutex

	trace func(string, []byte)
}

type readCloserWrapper struct {
	reader io.Reader
}

func (r *readCloserWrapper) Read(data []byte) (int, error) {
	return r.reader.Read(data)
}

func (r *readCloserWrapper) Close() error {
	return nil
}

type writeCloserWrapper struct {
	writer io.Writer
}

func (r *writeCloserWrapper) Write(data []byte) (int, error) {
	return r.writer.Write(data)
}

func (r *writeCloserWrapper) Close() error {
	return nil
}

func NewStreamPipe(reader io.ReadCloser, writer io.WriteCloser) *StreamPipe {
	return &StreamPipe{
		reader:     reader,
		writer:     writer,
		lineReader: bufio.NewReader(reader),
		trace:      func(string, []byte) {},
	}
}

func NewStreamPipeNoCloser(reader io.Reader, writer io.Writer) *StreamPipe {
	return NewStreamPipe(&readCloserWrapper{reader}, &writeCloserWrapper{writer})
}

func NewStreamPipeRW(rw io.ReadWriteCloser) *StreamPipe {
	return NewStreamPipe(&readCloserWrapper{rw}, rw)
}

func NewStreamPipeRWNoCloser(rw io.ReadWriter) *StreamPipe {
	return NewStreamPipe(&readCloserWrapper{rw}, &writeCloserWrapper{rw})
}

func (p *StreamPipe) Recv() (*Message, error) {
	for {
		if p.end {
			return nil, nil
		}
		if p.err != nil {
			p.end = true
			err := p.err
			p.err = nil
			return nil, err
		}

		data, err := p.lineReader.ReadBytes('\n')
		p.err = err
		if len(data) > 0 {
			msg := &Message{}
			if json.Unmarshal(data, msg) == nil {
				p.trace("<<", data)
				return msg, nil
			} else {
				p.trace("!<", data)
			}
		} else if err == nil {
			p.end = true
		}
	}
}

func (p *StreamPipe) Send(msg *Message, options ...interface{}) error {
	if content, err := json.Marshal(msg); err != nil {
		return err
	} else {
		p.writeLock.Lock()
		_, err := p.writer.Write(append(content, '\n'))
		p.writeLock.Unlock()
		if err == nil {
			p.trace(">>", content)
		} else {
			p.trace("!>", content)
		}
		return err
	}
}

func (p *StreamPipe) Close() error {
	p.reader.Close()
	p.writer.Close()
	return nil
}

func (p *StreamPipe) TraceOff() {
	p.trace = func(string, []byte) {}
}

func (p *StreamPipe) TraceOn(name string) {
	if name == "" {
		name = fmt.Sprintf("[%d] ", os.Getpid())
	} else {
		name += " "
	}
	p.trace = func(dir string, content []byte) {
		log.Println(name + dir + " " + string(content))
	}
}
