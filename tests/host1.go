package main

import (
	"../exts"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
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
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func runExt(name string) {
	log.Printf(name+" START [%d]\n", os.Getpid())
	ext := exts.NewExtension(name)
	ext.StreamPipe.Trace = true
	ext.Do("poll", func(p exts.MessagePipe, action string, data exts.RawMessage) (exts.RawMessage, error) {
		content, err := json.Marshal(&Payload{name, 1})
		go func() {
			ext.Notify("refresh", content)
		}()
		return content, err
	})
	ext.Run()
	log.Println(name + " EXIT")
}

func runHost(numExt, iterations int) {
	log.Printf("HOST START [%d]\n", os.Getpid())
	host := exts.NewExtensionHost()
	for i := 0; i < numExt; i++ {
		ext := host.Load(fmt.Sprintf("HOST-%d", i), os.Args[0], "-x", "-r", fmt.Sprintf("EXT-%d", i))
		ext.StreamPipe.Trace = true
	}
	host.Dispatcher.
		On("refresh", func(p exts.MessagePipe, action string, data exts.RawMessage) {
		val := &Payload{}
		if err := json.Unmarshal(data, val); err != nil {
			panic(err)
		}
		log.Printf("REFRESH %s: %v\n", val.Name, val.Value)
	})
	go func() {
		for i := 0; i < numExt; i++ {
			name := fmt.Sprintf("HOST-%d", i)
			val := &Payload{}
			if invoker := host.Dispatcher.Invoker(name); invoker != nil {
				exts.InvokeHelp(invoker).
					WithAction("poll").
					Invoke().
					Unmarshal(val)
				log.Printf("POLL %s: %v\n", name, val.Value)
			} else {
				log.Printf("INVOKER NIL %v\n", i)
			}
		}
		host.Close()
	}()
	host.Run()
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
