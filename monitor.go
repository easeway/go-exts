package exts

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

const (
	startRetries = 5
	retryDelay   = 100 * time.Millisecond
	eventEvent   = "E"
	eventInvoke  = "R"
)

var (
	errorInactive = errors.New("Extension is inactive")
	errorNotStart = errors.New("Process not started")
)

type extProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func startExtProcess(path string, args []string) (*extProcess, error) {
	process := &extProcess{
		cmd: exec.Command(path, args...),
	}

	var err error

	if process.stdin, err = process.cmd.StdinPipe(); err != nil {
		return nil, err
	}

	if process.stdout, err = process.cmd.StdoutPipe(); err != nil {
		process.Close()
		return nil, err
	}

	process.cmd.Stderr = os.Stderr
	if err = process.cmd.Start(); err != nil {
		process.Close()
		return nil, err
	}

	return process, nil
}

func (proc *extProcess) Close() {
	if proc.stdin != nil {
		proc.stdin.Close()
	}

	if proc.stdout != nil {
		proc.stdout.Close()
	}
}

func (proc *extProcess) Wait() {
	proc.Close()
	proc.cmd.Wait()
}

type response struct {
	data Reply
	err  error
}

type extMonitor struct {
	host *extsHost
	name string
	path string
	args []string

	invokeId uint32
	mutex    sync.Mutex
	cond     *sync.Cond
	process  *extProcess
	active   bool
	replies  map[uint32]*response
}

type message struct {
	Event string          `json:"event"`
	Id    uint32          `json:"id,omitempty"`
	Name  string          `json:"name"`
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
}

func encodeMessageRaw(event, name string, id uint32, data []byte) ([]byte, error) {
	if encoded, err := json.Marshal(&message{
		Event: event,
		Name:  name,
		Data:  data,
		Id:    id,
	}); err == nil {
		return append(encoded, '\n'), nil
	} else {
		return nil, err
	}
}

func encodeMessage(event, name string, id uint32, data interface{}) ([]byte, error) {
	if encoded, err := json.Marshal(data); err != nil {
		return nil, err
	} else {
		return encodeMessageRaw(event, name, id, encoded)
	}
}

func encodeMessageErr(event, name string, id uint32, data interface{}, e error) ([]byte, error) {
	if encoded, err := json.Marshal(data); err != nil {
		return nil, err
	} else {
		if encoded, err = json.Marshal(&message{
			Event: event,
			Name:  name,
			Data:  encoded,
			Error: e.Error(),
			Id:    id,
		}); err != nil {
			return nil, err
		}
		return append(encoded, '\n'), nil
	}
}

func newMonitor(host *extsHost, name, path string, args []string) *extMonitor {
	ext := &extMonitor{
		host:    host,
		name:    name,
		path:    path,
		args:    args,
		active:  true,
		replies: make(map[uint32]*response),
	}
	ext.cond = sync.NewCond(&ext.mutex)
	return ext
}

func (ext *extMonitor) isActive() bool {
	return ext.active && !ext.host.stopping
}

func (ext *extMonitor) emitError(name string, err error) {
	ext.host.pushEvent(&hostEvent{
		eventType: eventError,
		extEvent: &Event{
			Name: name,
			Ext:  ext,
		},
		extError: err,
	})
}

func (ext *extMonitor) start() error {
	process, err := startExtProcess(ext.path, ext.args)
	if err != nil {
		return err
	}

	ext.mutex.Lock()
	defer ext.mutex.Unlock()
	ext.process = process
	return nil
}

func (ext *extMonitor) tryToStart() error {
	var err error
	for i := 0; i < startRetries; i++ {
		if !ext.isActive() {
			return errorInactive
		}
		if err = ext.start(); err != nil {
			ext.emitError(ErrorStart, err)
		} else {
			break
		}
		time.Sleep(retryDelay)
	}
	return err
}

func (ext *extMonitor) waitProcess() {
	ext.mutex.Lock()
	process := ext.process
	ext.process = nil
	ext.active = false
	ext.mutex.Unlock()
	if process != nil {
		process.Wait()
	}
}

func (ext *extMonitor) processOutput() {
	defer ext.waitProcess()

	reader := bufio.NewReader(ext.process.stdout)
	for ext.isActive() {
		line, err := reader.ReadString('\n')

		var msg message
		if json.Unmarshal([]byte(line), &msg) == nil {
			log.Println(ext.name + " << " + line)
			var data []byte = msg.Data
			switch msg.Event {
			case eventEvent:
				ext.host.pushEvent(&hostEvent{
					eventType: eventData,
					extEvent: &Event{
						Name: msg.Name,
						Data: data,
						Ext:  ext,
					},
				})
			case eventInvoke:
				ext.mutex.Lock()
				if resp, exists := ext.replies[msg.Id]; exists {
					resp.data = data
					if len(msg.Error) > 0 {
						resp.err = errors.New(msg.Error)
					}
					ext.cond.Broadcast()
				}
				ext.mutex.Unlock()
			}
		} else if err == nil {
			log.Println(ext.name + " !! " + line)
		}

		if err != nil {
			if ext.isActive() {
				ext.emitError(ErrorComm, err)
			}
			break
		}
	}
}

func (ext *extMonitor) send(data []byte) error {
	log.Println(ext.name + " >> " + string(data))
	_, err := ext.process.stdin.Write(data)
	return err
}

func (ext *extMonitor) run() {
	for ext.isActive() {
		if err := ext.tryToStart(); err != nil {
			ext.emitError(ErrorLoad, err)
			break
		}

		ext.processOutput()
	}

	ext.mutex.Lock()
	ext.active = false
	ext.cond.Broadcast()
	ext.mutex.Unlock()

	ext.host.remove(ext.name)
	ext.host.extsWg.Done()
}

func (ext *extMonitor) Name() string {
	return ext.name
}

func (ext *extMonitor) Notify(event string, data interface{}) error {
	ext.mutex.Lock()
	defer ext.mutex.Unlock()
	if !ext.isActive() {
		return errorInactive
	}
	if ext.process == nil {
		return errorNotStart
	}
	encoded, err := encodeMessage(eventEvent, event, 0, data)
	if err == nil {
		err = ext.send(encoded)
	}
	return err
}

func (ext *extMonitor) Invoke(action string, params interface{}) (Reply, error) {
	if encoded, err := json.Marshal(params); err == nil {
		return ext.InvokeRaw(action, encoded)
	} else {
		return nil, err
	}
}

func (ext *extMonitor) InvokeRaw(action string, params []byte) (Reply, error) {
	ext.mutex.Lock()
	defer ext.mutex.Unlock()

	if !ext.isActive() {
		return nil, errorInactive
	}
	if ext.process == nil {
		return nil, errorNotStart
	}

	resp := &response{}
	id := atomic.AddUint32(&ext.invokeId, 1)

	encoded, err := encodeMessageRaw(eventInvoke, action, id, params)
	if err == nil {
		ext.replies[id] = resp
		err = ext.send(encoded)
	}

	if err == nil {
		err = errorInactive
		for ext.isActive() {
			ext.cond.Wait()
			if resp.data != nil || resp.err != nil {
				err = resp.err
				break
			}
		}
	}
	delete(ext.replies, id)
	return resp.data, err
}

func (ext *extMonitor) Unload() error {
	ext.mutex.Lock()
	if ext.active {
		ext.active = false
		ext.cond.Wait()
	}
	ext.mutex.Unlock()
	return nil
}
