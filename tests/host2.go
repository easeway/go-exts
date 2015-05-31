package main

import (
	"../exts"
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

func runExt() {
	log.Printf("EXT START [%d]\n", os.Getpid())
	quitCh := make(chan interface{})
	ext := exts.NewExtension()
	ext.StreamPipe.TraceOn("EXT")
	ext.Do("quit", func(p exts.MessagePipe, action string, data exts.RawMessage) (exts.RawMessage, error) {
		log.Println("QUITING ...")
		quitCh <- nil
		return nil, nil
	})
	go func() {
		ext.NotifyHelp().
			WithEvent("hello").
			Notify()
	}()
	go ext.Run()
	<-quitCh
	log.Println("EXT EXIT")
}

func runHost() {
	log.Printf("HOST START [%d]\n", os.Getpid())
	host := exts.NewExtensionHost()
	ext := host.Load("EXT", os.Args[0], "-x")
	ext.StreamPipe.TraceOn("HOST.X")
	ext.Start()

	host.Dispatcher.
		On("hello", func(p exts.MessagePipe, action string, data exts.RawMessage) {
		exts.InvokeHelp(&exts.PipeInvoker{p}).WithAction("quit").Invoke()
	})
	host.Run()
	log.Println("HOST EXIT")
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
