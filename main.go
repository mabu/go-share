package main

import (
	"flag"
	"log"
	"github.com/mabu/go-share/share"
	"github.com/mewpkg/gopass"
	"os"
	"os/signal"
)

func main() {
	port := flag.Int("p", 80, "port number")
	directory := flag.String("d", ".", "directory for uploaded files")
	flag.Parse()

	password, err := gopass.GetPass("Please enter a password for file upload: ")
	if err != nil {
		log.Fatalln("Error:", err)
	}
	if p, err := gopass.GetPass("Please repeat the password: "); err != nil {
		log.Fatalln("Error:", err)
	} else if password != p {
		log.Fatalln("Passwords do not match.")
	}

	c := make(chan os.Signal)
	signal.Notify(c)
	go func() {
		sig := <-c
		log.Fatalf("Caught signal %v, exiting...\n", sig)
	}()

	s, err := share.New(*directory, password)
	if err != nil {
		log.Fatalln("Could not create server:", err)
	}
	log.Printf("Starting go-share on port %d.\n", *port)
	if err := s.Start(*port); err != nil {
		log.Fatalln("Could not start server:", err)
	}
}
