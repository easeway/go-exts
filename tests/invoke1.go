package main

import (
	"../exts"
	"encoding/json"
	"flag"
	"log"
	"os"
	"sync"
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

func runExt() {
	log.Printf("EXTS START [%d]\n", os.Getpid())
	p := exts.NewStreamPipeNoCloser(os.Stdin, os.Stdout)
	p.TraceOn("EXTS")
	v := exts.NewInvokerPipe(p)
	d := exts.NewDispatchPipe(v)
	d.On("start", func(p exts.MessagePipe, event string, data exts.RawMessage) {
		val := &Payload{}
		if err := json.Unmarshal(data, &val); err != nil {
			panic(err)
		}
		val.Value++
		log.Printf("EXTS PING WITH %v\n", val.Value)
		err := exts.InvokeHelp(v).
			WithAction("ping").
			Marshal(val).
			Invoke().
			Unmarshal(val)
		if err != nil {
			panic(err)
		}
		log.Printf("EXTS BYE WITH %v\n", val.Value)
		err = exts.InvokeHelp(v).
			WithAction("bye").
			Marshal(val).
			Invoke().
			Error
		if err != nil {
			panic(err)
		}
	}).On("bye", func(p exts.MessagePipe, event string, data exts.RawMessage) {
		log.Println("EXTS BYE")
		d.Close()
	})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		exts.RunPipe(d)
		wg.Done()
	}()

	log.Println("EXTS NOTIFY")
	err := exts.NotifyHelp(v).
		WithEvent("ext").
		Notify()
	if err != nil {
		panic(err)
	}

	wg.Wait()
	log.Printf("EXTS EXIT")
}

func runHost() {
	log.Printf("HOST START [%d]\n", os.Getpid())
	s := exts.NewRespawnProcStream(os.Args[0], "-x")
	s.Cmd.Stderr = os.Stdout
	p := s.Pipe()
	p.TraceOn("HOST")
	v := exts.NewInvokerPipe(p)
	d := exts.NewDispatchPipe(v)
	d.Do("ping", func(p exts.MessagePipe, action string, data exts.RawMessage) (exts.RawMessage, error) {
		val := &Payload{}
		if err := json.Unmarshal(data, &val); err != nil {
			return nil, err
		}
		val.Value++
		log.Printf("HOST PING TO %v\n", val.Value)
		return json.Marshal(val)
	}).Do("bye", func(p exts.MessagePipe, action string, data exts.RawMessage) (exts.RawMessage, error) {
		val := &Payload{}
		if err := json.Unmarshal(data, &val); err != nil {
			return nil, err
		}
		log.Println("HOST BYE INVOKED")
		if val.Value == 3 {
			exts.NotifyHelp(v).
				WithEvent("bye").
				Notify()
			d.Close()
		}
		return nil, nil
	}).On("ext", func(p exts.MessagePipe, action string, data exts.RawMessage) {
		log.Println("HOST NOTIFY")
		err := exts.NotifyHelp(v).
			WithEvent("start").
			Marshal(&Payload{Value: 1}).
			Notify()
		if err != nil {
			panic(err)
		}
	})

	s.Start()
	exts.RunPipe(d)
	log.Printf("HOST EXIT")
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
