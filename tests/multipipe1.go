package main

import (
	"../exts"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
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

func run(name string, p *exts.StreamPipe) {
	log.Printf(name+" START [%d]\n", os.Getpid())
	p.Trace = true
	p.TraceName = name + " "
	v := exts.NewInvokerPipeRunner(p)
	d := exts.NewDispatchPipeRunner(v.Pipe())
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
		d.Pipe().Close()
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
				d.Pipe().Close()
			}
		}
	}()
	d.Pipe().Run()
	log.Printf(name + " EXIT")

}

func runExt(name string) {
	p := exts.NewStreamPipeNoCloser(os.Stdin, os.Stdout)
	log.Printf(name+" START [%d]\n", os.Getpid())
	p.Trace = true
	p.TraceName = name + " "
	v := exts.NewInvokerPipeRunner(p)
	d := exts.NewDispatchPipeRunner(v.Pipe())
	val := &Payload{0}
	d.On("reset", func(p exts.MessagePipe, event string, data exts.RawMessage) {
		val.Value = 0
	}).Do("report", func(p exts.MessagePipe, action string, data exts.RawMessage) (exts.RawMessage, error) {
		reportVal := &Payload{}
		if err := json.Unmarshal(data, reportVal); err != nil {
			panic(err)
		}
		log.Printf(name+" REPORT %v\n", reportVal.Value)
		return nil, nil
	})
	go func() {
		for {
			err := exts.InvokeHelp(v).
				WithAction("incr").
				Marshal(val).
				Invoke().
				Unmarshal(val)
			if err != nil {
				panic(err)
			}
			//time.Sleep(time.Second)
		}
	}()
	d.Pipe().Run()
	log.Println(name + " EXIT")
}

func runHost(numExt, iterations int) {
	log.Printf("HOST START [%d]\n", os.Getpid())
	d := exts.NewMultiPipeDispatcher()
	for i := 0; i < numExt; i++ {
		s := exts.NewRespawnProcStream(os.Args[0], "-x", "-r", fmt.Sprintf("EXT-%d", i))
		//s.Cmd.Stderr = os.Stdout
		p := s.Pipe()
		p.Trace = true
		p.TraceName = fmt.Sprintf("HOST-%d ", i)
		v := exts.NewInvokerPipeRunner(p)
		d.AddPipe(p.TraceName, v.Pipe())
		s.Start()
	}
	d.Do("incr", func(p exts.MessagePipe, action string, data exts.RawMessage) (exts.RawMessage, error) {
		val := &Payload{}
		if err := json.Unmarshal(data, val); err != nil {
			panic(err)
		}
		err := exts.InvokeHelp(&exts.PipeInvoker{p}).
			WithAction("report").
			Marshal(val).
			Invoke().
			Error
		if err != nil {
			panic(err)
		}
		val.Value++
		return json.Marshal(val)
	})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for i := 0; i < iterations; i++ {
			<-ticker.C
			d.Broadcast("reset", nil)
		}
		d.Close()
	}()
	d.Run()
	log.Println("HOST EXIT")
}

func main() {
	asExt := false
	numExt := 16
	identifier := "EXTS"
	iterations := 100
	flag.BoolVar(&asExt, "x", asExt, "As extension")
	flag.IntVar(&numExt, "n", numExt, "Number of extensions")
	flag.StringVar(&identifier, "r", identifier, "Name of extension")
	flag.IntVar(&iterations, "c", iterations, "Interations")
	flag.Parse()

	if asExt {
		runExt(identifier)
	} else {
		runHost(numExt, iterations)
	}
}
