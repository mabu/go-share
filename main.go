package main

import (
	"flag"
	"fmt"
	"github.com/mabu/go-share/share"
	"github.com/mewpkg/gopass"
	"os"
	"os/signal"
)

func printAndExit(msgs ...interface{}) {
	fmt.Println(msgs...)
	os.Exit(1)
}

func main() {
	port := flag.Int("p", 80, "port number")
	directory := flag.String("d", ".", "directory for uploaded files")
	flag.Parse()

	password, err := gopass.GetPass("Please enter a password for file upload: ")
	if err != nil {
		printAndExit("Error:", err)
	}
	if p, err := gopass.GetPass("Please repeat the password: "); err != nil {
		printAndExit("Error:", err)
	} else if password != p {
		printAndExit("Passwords do not match.")
	}

	c := make(chan os.Signal)
	signal.Notify(c)
	go func() {
		sig := <-c
		fmt.Printf("Caught signal %v, exiting...", sig)
		os.Exit(0)
	}()

	s, err := share.New(*directory, password)
	if err != nil {
		printAndExit("Could not create server:", err)
	}
	fmt.Printf("Starting go-share on port %d.\n", *port)
	if err := s.Start(*port); err != nil {
		printAndExit("Could not start server:", err)
	}
}
