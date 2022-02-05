package main

import (
	"flag"
	"log"
)

func get_args() {
	HostPtr = flag.String("host", "", "remote host to connect to")
	PortPtr = flag.String("port", "22", "remote port of sshd")
	FilesystemPtr = flag.String("filesystem", "", "The remote filesystem to be resized")
	SizePtr = flag.Int("size", 0, "New target size in Gb for remote filesystem")
	DryRunPtr = flag.Bool("dry-run", false, "Dry-Run, do not increase filesystem")
	flag.Parse()
	if *HostPtr == "" {
		log.Fatal("host parameter can not be empty")
	}
}
