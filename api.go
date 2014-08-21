package exts

import (
	"encoding/json"
	"errors"
)

const (
	ErrorStart = "START"
	ErrorLoad  = "LOAD"
	ErrorComm  = "COMM"
)

type Reply []byte

func (r Reply) Parse(v interface{}) error {
	return json.Unmarshal(r, v)
}

type Event struct {
	Name string
	Data Reply
	Ext  *Extension
}

type Extension interface {
	Notify(event string, data interface{}) error
	Invoke(action string, params interface{}) (Reply, error)
	Unload() error
}

func (ext *Extension) InvokeAndParse(action string, params, result interface{}) error {
	if reply, err := ext.Invoke(action, params); err == nil {
		return reply.Parse(result)
	} else {
		return err
	}
}

type ExtsHost interface {
	Load(name, path string, args ...string) (*Extension, error)
	Find(name string) *Extension
	WaitEvent() (*Event, error)
	Shutdown()
}

var errorExtNotLoaded = errors.New("Extension not loaded")

func (host *ExtsHost) Notify(extName, event string, data interface{}) error {
	if ext := host.Find(extName); ext != nil {
		return ext.Notify(event, data)
	} else {
		return errorExtNotLoaded
	}
}

func (host *ExtsHost) Invoke(extName, action string, params interface{}) (Reply, error) {
	if ext := host.Find(extName); ext != nil {
		return ext.Invoke(action, params)
	} else {
		return errorExtNotLoaded
	}
}

func (h *ExtsHost) InvokeAndParse(extName, action string, params, result interface{}) error {
	if ext := host.Find(extName); ext != nil {
		return ext.InvokeAndParse(action, params, result)
	} else {
		return errorExtNotLoaded
	}
}
