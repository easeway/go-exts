package exts

import (
	"io"
	"os/exec"
	"time"
)

type ProcStream struct {
	Cmd *exec.Cmd

	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func ProcStreamFromCmd(cmd *exec.Cmd) (s *ProcStream, err error) {
	s = &ProcStream{Cmd: cmd}
	if s.stdin, err = s.Cmd.StdinPipe(); err != nil {
		return nil, err
	}
	if s.stdout, err = s.Cmd.StdoutPipe(); err != nil {
		s.stdout.Close()
		return nil, err
	}
	return s, nil
}

func NewProcStream(prog string, args ...string) (s *ProcStream, err error) {
	return ProcStreamFromCmd(exec.Command(prog, args...))
}

func (s *ProcStream) Read(data []byte) (int, error) {
	return s.stdout.Read(data)
}

func (s *ProcStream) Write(data []byte) (int, error) {
	return s.stdin.Write(data)
}

func (s *ProcStream) Close() error {
	s.stdin.Close()
	s.stdout.Close()
	return nil
}

func (s *ProcStream) Start() error {
	if err := s.Cmd.Start(); err != nil {
		return err
	}
	s.Cmd.Process.Release()
	return nil
}

func (s *ProcStream) Pipe() *StreamPipe {
	return NewStreamPipeRW(s)
}

func NewProcPipe(prog string, args ...string) (MessagePipe, error) {
	if s, err := NewProcStream(prog, args...); err != nil {
		return nil, err
	} else {
		return s.Pipe(), nil
	}
}

type RespawnProcStream struct {
	Cmd             *exec.Cmd
	RespawnInternal time.Duration

	proc    *ProcStream
	ctrlCh  chan func()
	closeCh chan interface{}
}

func RespawnProcStreamFromCmd(cmd *exec.Cmd) *RespawnProcStream {
	return &RespawnProcStream{
		Cmd:     cmd,
		ctrlCh:  make(chan func()),
		closeCh: make(chan interface{}),
	}
}

func NewRespawnProcStream(prog string, args ...string) *RespawnProcStream {
	return RespawnProcStreamFromCmd(exec.Command(prog, args...))
}

func (s *RespawnProcStream) performIo(iofn func(proc *ProcStream) (int, error), nilErr error) (int, error) {
	for {
		if proc := s.currentProc(); proc != nil {
			if n, err := iofn(proc); err == nil {
				return n, err
			} else if n > 0 {
				return n, nil
			} else if proc.Cmd.Process != nil {
				// process still running, unable to communicate
				// kill the process and wait until it starts again
				proc.Cmd.Process.Kill()
			}
		} else {
			return 0, nilErr
		}
	}
}

func (s *RespawnProcStream) Read(data []byte) (int, error) {
	return s.performIo(func(proc *ProcStream) (int, error) {
		return proc.Read(data)
	}, io.EOF)
}

func (s *RespawnProcStream) Write(data []byte) (int, error) {
	return s.performIo(func(proc *ProcStream) (int, error) {
		return proc.Write(data)
	}, io.ErrClosedPipe)
}

func (s *RespawnProcStream) Close() error {
	s.closeCh <- nil
	return nil
}

func (s *RespawnProcStream) Start() {
	go s.Run()
}

func (s *RespawnProcStream) Run() {
	notify := make(chan error)
	defer close(s.ctrlCh)
	for {
		cmd := *s.Cmd
		var err error = nil
		if s.proc, err = ProcStreamFromCmd(&cmd); err == nil {
			go func(proc *ProcStream) {
				notify <- proc.Cmd.Run()
			}(s.proc)
			for running := true; running; {
				select {
				case <-notify:
					running = false
				case fn := <-s.ctrlCh:
					fn()
				case <-s.closeCh:
					if s.proc != nil {
						s.proc.Close()
						s.proc = nil
					}
					return
				}
			}
			s.proc = nil
		}
		interval := s.RespawnInternal
		if interval == 0 {
			interval = time.Second
		}
		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
		case <-s.closeCh:
			return
		}
	}
}

func (s *RespawnProcStream) RespawnInterval(timeout time.Duration) *RespawnProcStream {
	s.RespawnInternal = timeout
	return s
}

func (s *RespawnProcStream) Pipe() *StreamPipe {
	return NewStreamPipeRW(s)
}

func (s *RespawnProcStream) currentProc() *ProcStream {
	defer func() {
		recover()
	}()
	ch := make(chan *ProcStream)
	s.ctrlCh <- func() {
		ch <- s.proc
	}
	return <-ch
}
