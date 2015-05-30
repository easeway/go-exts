package exts

import (
	"io"
)

type PipeRunner interface {
	io.Closer
	Pipe() MessagePipe
	Recv(*PipeBase, *RecvPacket) *RecvPacket
	Send(*PipeBase, *SendPacket) *SendPacket
}

type PipeBase struct {
	Source MessagePipe
	RecvCh chan *RecvPacket
	SendCh chan *SendPacket
	CtrlCh chan func()
	Stop   bool
	runner PipeRunner
}

func UnconnectedEnd(pipe MessagePipe) {
	ch := pipe.RecvChan()
	go func() {
		for {
			if _, ok := <-ch; !ok {
				break
			}
		}
	}()
}

func ConnectPipe(runner PipeRunner, source MessagePipe) *PipeBase {
	return &PipeBase{
		Source: source,
		RecvCh: make(chan *RecvPacket),
		SendCh: make(chan *SendPacket),
		CtrlCh: make(chan func()),
		runner: runner,
	}
}

func (p *PipeBase) RecvChan() <-chan *RecvPacket {
	return p.RecvCh
}

func (p *PipeBase) Run() {
	go p.Source.Run()
	defer func() {
		p.Source.Close()
		p.runner.Close()
	}()
	for !p.Stop {
		select {
		case r, ok := (<-p.Source.RecvChan()):
			if ok {
				if r.Error == nil && r.Message != nil {
					pkt := p.runner.Recv(p, r)
					if pkt != nil {
						p.RecvCh <- pkt
					}
				}
			}
		case s, ok := (<-p.SendCh):
			if ok {
				pkt := p.runner.Send(p, s)
				if pkt != nil {
					s.Result <- p.Source.Send(s.Message)
				}
			}
		case fn, ok := (<-p.CtrlCh):
			if !ok {
				return
			} else {
				fn()
			}
		}
	}
}

func (p *PipeBase) Send(msg *Message, options ...interface{}) *SendReceipt {
	pkg := &SendPacket{
		Message: msg,
		Options: options,
		Result:  make(chan *SendReceipt),
	}
	p.SendCh <- pkg
	return <-pkg.Result
}

func (p *PipeBase) Close() error {
	close(p.CtrlCh)
	return nil
}

func (p *PipeBase) RunCtrl(fn func()) {
	ch := make(chan interface{})
	defer func() {
		recover()
	}()
	p.CtrlCh <- func() {
		fn()
		ch <- nil
	}
	<-ch
}