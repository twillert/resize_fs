package main

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

func check_remote_lvm(devstring string) (err error) {
	// is the remote machine using LVM for the filesystem?
	if !strings.HasPrefix(devstring, "/dev/mapper") {
		return fmt.Errorf("No /dev/mapper device detected, not a LVM filesystem.")
	}
	return
}

func check_remote_commands(c *ssh.Client) (err error) {
	// does the remote Linux machine have all requirement commands installed?
	for _, v := range bin {
		//fmt.Println("checking: ", k, v)
		_, err = remote_exec(c, "/usr/bin/test -x "+v)
		if err != nil {
			return err
		}
	}
	return
}
