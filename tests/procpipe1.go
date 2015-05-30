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
	log.Println("EXTS START")
	p := exts.NewStreamPipeNoCloser(os.Stdin, os.Stdout)
	for {
		msg, err := p.Recv()
		if msg == nil {
			break
		}
		if err != nil {
			panic(err)
		}
		printMsg("EXTS", msg)
		if err := p.Send(msg); err != nil {
			panic(err)
		}
	}
	p.Close()
	log.Printf("EXTS EXIT")
}

func runHost() {
	log.Println("HOST START")
	s := exts.NewRespawnProcStream(os.Args[0], "-x")
	s.Cmd.Stderr = os.Stdout
	p := s.Pipe()
	s.Start()
	go func() {
		log.Println("HOST SEND")
		receipt := p.Send(&exts.Message{
			Event: exts.MsgNotify,
			Id:    1,
			Name:  "test",
		})
		if receipt != nil && receipt.Error != nil {
			panic(receipt.Error)
		}
	}()
	for {
		msg, err := p.Recv()
		if msg == nil {
			break
		}
		if err != nil {
			panic(err)
		}
		printMsg("HOST", msg)
		msg.Id++
		if err := p.Send(msg); err != nil {
			panic(err)
		}
		if msg.Id >= 1000 {
			break
		}
	}
	p.Close()
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
