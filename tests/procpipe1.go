package main

import (
	"../exts"
	"flag"
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

func runExt() {
	log.Println("EXTS START")
	p := exts.NewStreamPipeNoCloser(os.Stdin, os.Stdout)
	go p.Run()
	for {
		if pkt, ok := <-p.RecvChan(); ok {
			printMsg("EXTS", pkt)
			if pkt.Message != nil {
				if receipt := p.Send(pkt.Message); receipt != nil && receipt.Error != nil {
					log.Printf("EXTS SEND ERROR: %v\n", receipt.Error)
				}
			} else if pkt.Error != nil {
				p.Close()
			}
		} else {
			break
		}
	}
	log.Printf("EXTS EXIT")
}

func runHost() {
	log.Println("HOST START")
	s := exts.NewRespawnProcStream(os.Args[0], "-x")
	s.Cmd.Stderr = os.Stdout
	p := s.Pipe()
	s.Start()
	go p.Run()
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
		if pkt, ok := <-p.RecvChan(); ok {
			printMsg("HOST", pkt)
			if pkt.Message != nil {
				pkt.Message.Id++
				if r := p.Send(pkt.Message); r != nil && r.Error != nil {
					log.Printf("HOST SEND ERROR: %v\n", r.Error)
				}
				if pkt.Message.Id >= 1000 {
					p.Close()
				}
			}
		} else {
			break
		}

	}
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
