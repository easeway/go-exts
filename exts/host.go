package exts

import (
	"os"
	"os/exec"
)

type ExtensionHost struct {
	Dispatcher MultiPipeDispatcher
}

type LoadedExtension struct {
	Stream     *RespawnProcStream
	StreamPipe *StreamPipe
	Invoker    Invoker
	Pipe       MessagePipe
}

func NewExtensionHost() *ExtensionHost {
	return &ExtensionHost{NewMultiPipeDispatcher()}
}

func (h *ExtensionHost) LoadCmd(name string, cmd *exec.Cmd) *LoadedExtension {
	ext := &LoadedExtension{}
	ext.Stream = RespawnProcStreamFromCmd(cmd)
	ext.Stream.Cmd.Stderr = os.Stdout
	ext.StreamPipe = ext.Stream.Pipe()
	v := NewInvokerPipe(ext.StreamPipe)
	ext.Invoker = v
	ext.Pipe = v
	h.Dispatcher.AddPipe(name, ext.Pipe)
	return ext
}

func (h *ExtensionHost) Load(name, prog string, args ...string) *LoadedExtension {
	return h.LoadCmd(name, exec.Command(prog, args...))
}

func (h *ExtensionHost) LoadCommandLine(name, cmdline string) *LoadedExtension {
	args := SplitCommandLine(cmdline)
	return h.Load(name, args[0], args[1:]...)
}

func (h *ExtensionHost) Run() {
	h.Dispatcher.Run()
}

func (h *ExtensionHost) Close() error {
	return h.Dispatcher.Close()
}

func (ext *LoadedExtension) Start() {
	ext.Stream.Start()
}

func (ext *LoadedExtension) Run() {
	ext.Stream.Run()
}

func SplitCommandLine(cmd string) []string {
	tokens := make([]string, 0)
	quot := ' '
	token := ""
	for i := 0; i < len(cmd); i++ {
		switch cmd[i] {
		case '\'':
			if quot == '\'' {
				quot = ' '
			} else if quot == '"' {
				token += string(cmd[i])
			} else {
				quot = '\''
			}
		case '"':
			if quot == '"' {
				quot = ' '
			} else if quot == '\'' {
				token += string(cmd[i])
			} else {
				quot = '"'
			}
		case ' ':
			if quot != ' ' {
				token += string(cmd[i])
			} else if len(token) > 0 {
				tokens = append(tokens, token)
				token = ""
			}
		default:
			token += string(cmd[i])
		}
	}
	if len(token) > 0 {
		tokens = append(tokens, token)
	}
	return tokens
}
