package main

import (
	"flag"
	"github.com/mewpkg/gopass"
	"log"
	"os"
	"os/signal"

	"github.com/mabu/go-share/share"
	"github.com/mabu/go-share/share/storage"
)

func main() {
	port := flag.Int("p", 80, "port number")
	directory := flag.String("d", "", "directory for uploaded files (default: create a temporary directory)")
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

	st, err := storage.NewDirectory(*directory)
	if err != nil {
		log.Fatalln("Could not create storage:", err)
	}
	log.Printf("Starting go-share on port %d.\n%v\n", *port, st)
	if err := share.New(st, password).Start(*port); err != nil {
		log.Fatalln("Could not start server:", err)
	}
}
