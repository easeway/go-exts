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

func runExt(name string) {
	p := exts.NewStreamPipeNoCloser(os.Stdin, os.Stdout)
	log.Printf(name+" START [%d]\n", os.Getpid())
	p.TraceOn(name)
	v := exts.NewInvokerPipe(p)
	d := exts.NewDispatchPipe(v)
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
	}).Do("sample", func(p exts.MessagePipe, event string, data exts.RawMessage) (exts.RawMessage, error) {
		return json.Marshal(&name)
	})
	go func() {
		for {
			err := exts.InvokeHelp(v).
				WithAction("incr").
				Marshal(val).
				Invoke().
				Unmarshal(val)
			if err != nil {
				log.Println(err)
			}
			//time.Sleep(time.Second)
		}
	}()
	exts.RunPipe(d)
	log.Println(name + " EXIT")
}

func runHost(numExt, iterations int) {
	log.Printf("HOST START [%d]\n", os.Getpid())
	d := exts.NewMultiPipeDispatcher()
	for i := 0; i < numExt; i++ {
		s := exts.NewRespawnProcStream(os.Args[0], "-x", "-r", fmt.Sprintf("EXT-%d", i))
		//s.Cmd.Stderr = os.Stdout
		p := s.Pipe()
		name := fmt.Sprintf("HOST-%d ", i)
		p.TraceOn(name)
		v := exts.NewInvokerPipe(p)
		d.AddPipe(name, v)
		s.Start()
	}
	d.Do("incr", func(p exts.MessagePipe, action string, data exts.RawMessage) (exts.RawMessage, error) {
		val := &Payload{}
		if err := json.Unmarshal(data, val); err != nil {
			log.Println(err)
		}
		err := exts.InvokeHelp(&exts.PipeInvoker{p}).
			WithAction("report").
			Marshal(val).
			Invoke().
			Error
		if err != nil {
			log.Println(err)
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
