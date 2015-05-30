package main

import (
	"../exts"
	"encoding/json"
	"flag"
	"log"
	"os"
)

func printMsg(prefix string, msg *exts.Message) {
	log.Printf(prefix+" < %s %v %s %s %s\n",
		msg.Event,
		msg.Id,
		msg.Name,
		string(msg.Data),
		msg.Error,
	)
}

type Payload struct {
	Value int `json:"value"`
}

func run(name string, p *exts.StreamPipe) {
	log.Printf(name+" START [%d]\n", os.Getpid())
	p.TraceOn(name)
	v := exts.NewInvokerPipe(p)
	d := exts.NewDispatchPipe(v)
	d.Do("ping", func(p exts.MessagePipe, action string, data exts.RawMessage) (exts.RawMessage, error) {
		val := &Payload{}
		if err := json.Unmarshal(data, &val); err != nil {
			return nil, err
		}
		log.Printf(name+" %s %v\n", action, val.Value)
		val.Value++
		return json.Marshal(val)
	}).On("bye", func(p exts.MessagePipe, action string, data exts.RawMessage) {
		log.Println(name + " BYE")
		exts.NotifyHelp(v).
			WithEvent("bye").
			Notify()
		d.Close()
	})

	go func() {
		val := &Payload{0}
		for {
			err := exts.InvokeHelp(v).
				WithAction("ping").
				Marshal(val).
				Invoke().
				Unmarshal(val)
			if err != nil {
				log.Printf(name+" ERR: %v\n", err)
			}
			val.Value++
			if val.Value > 1000 {
				err = exts.NotifyHelp(v).
					WithEvent("bye").
					Notify()
				if err != nil {
					log.Printf(name+" ERR: %v\n", err)
				}
				d.Close()
			}
		}
	}()
	exts.RunPipe(d)
	log.Printf(name + " EXIT")
}

func runExt() {
	p := exts.NewStreamPipeNoCloser(os.Stdin, os.Stdout)
	run("EXTS", p)
}

func runHost() {
	s := exts.NewRespawnProcStream(os.Args[0], "-x")
	s.Cmd.Stderr = os.Stdout
	s.Start()
	run("HOST", s.Pipe())
}

func main() {
	asExt := false
	flag.BoolVar(&asExt, "x", asExt, "As extension")
	flag.Parse()

	if asExt {
		runExt()
	} else {
		runHost()
	}
}
