package main

import (
	"../exts"
	"encoding/json"
	"flag"
	"log"
	"os"
	"sync"
)

func printMsg(prefix string, pkt *exts.RecvPacket) {
	if pkt.Message != nil {
		log.Printf(prefix+" < %s %v %s %s %s\n",
			pkt.Message.Event,
			pkt.Message.Id,
			pkt.Message.Name,
			pkt.Message.Data,
			pkt.Message.Error,
		)
	} else if pkt.Error != nil {
		log.Printf(prefix+" RECV ERROR: %v\n", pkt.Error)
	} else if pkt.Raw != nil {
		log.Printf(prefix+" ! %s\n", string(pkt.Raw))
	}
}

type Payload struct {
	Value int `json:"value"`
}

func runExt() {
	log.Printf("EXTS START [%d]\n", os.Getpid())
	p := exts.NewStreamPipeNoCloser(os.Stdin, os.Stdout)
	p.Trace = true
	v := exts.NewInvokerPipeRunner(p)
	d := exts.NewDispatchPipeRunner(v.Pipe())
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
		d.Pipe().Close()
	})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		d.Pipe().Run()
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
	s.Pipe().Trace = true
	s.Cmd.Stderr = os.Stdout
	v := exts.NewInvokerPipeRunner(s.Pipe())
	d := exts.NewDispatchPipeRunner(v.Pipe())
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
			d.Pipe().Close()
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
	d.Pipe().Run()
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
