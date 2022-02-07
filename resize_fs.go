package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

var HostPtr, PortPtr, FilesystemPtr, UserPtr *string
var SizePtr *int
var DryRunPtr *bool

var bin = map[string]string{
	"df":       "/usr/bin/df",
	"grep":     "/usr/bin/grep",
	"lvresize": "/usr/sbin/lvresize",
	"lsscsi":   "/usr/bin/lsscsi",
	"lvs":      "/usr/sbin/lvs",
	"pvcreate": "/usr/sbin/pvcreate",
	"pvs":      "/usr/sbin/pvs",
	"tail":     "/usr/bin/tail",
	"vgextend": "/usr/sbin/vgextend",
}

func remote_exec(c *ssh.Client, cmd string) (out string, err error) {
	s, err := c.NewSession()
	if err != nil {
		log.Fatal("Failed to create session: ", err)
	}
	defer s.Close()

	outbyte, err := s.CombinedOutput(cmd)
	out = string(outbyte)
	return
}

func get_scsi_devices(c *ssh.Client) string {
	output, err := remote_exec(c, "/usr/bin/lsscsi | grep -E -o '/dev/[[:alnum:]]+'")
	if err != nil {
		log.Fatal("Could not get scsi devices: " + output)
	}
	return output
}

func get_single_new_device(a1, a2 []string) (newdevice string) {
	// length of arrays should be off be precisely one
	diff := len(a1) - len(a2)
	if (diff != 1) && (diff != -1) {
		log.Fatal("Array lengths are not off by one...", a1, a2)
	}
	m := make(map[string]int)
	for _, value := range a1 {
		m[value]++
	}
	for _, value := range a2 {
		m[value]++
	}
	found := false
	for index := range m {
		if m[index] == 1 {
			newdevice = index
			found = true
		}
	}
	if !found {
		log.Fatal("No new device found: ", m)
	}
	return newdevice
}

func get_vg_and_lv(dev_mapper_device string) (vg, lv string) {
	// assume dev_mapper_device looks like this: /dev/mapper/vg_system-var
	dev_mapper_device = strings.TrimPrefix(dev_mapper_device, "/dev/mapper/")
	s := strings.SplitN(dev_mapper_device, "-", 2)
	vg = s[0]
	lv = s[1]
	return
}

func main() {

	var pw string = os.Getenv("pw")
	var token string = os.Getenv("token")
	if pw == "" {
		log.Fatal("'pw' environment variable not set")
	}
	if token == "" {
		log.Fatal("'token' environment variable not set")
	}

	get_args() // get command line paramters

	config := &ssh.ClientConfig{
		User: *UserPtr,
		Auth: []ssh.AuthMethod{
			ssh.Password(pw),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	client, err := ssh.Dial("tcp", *HostPtr+":"+*PortPtr, config)
	if err != nil {
		log.Fatal("Failed to dial: ", err)
	}
	defer client.Close()
	err = check_remote_commands(client)
	if err != nil {
		log.Fatal("Failed to check remote commands: ", err)
	}

	// check filesystem and get device+size for remote filesystem
	// use "df -BG" to enforce consistent size format
	output, err := remote_exec(client, "/usr/bin/df -BG "+*FilesystemPtr+"| /usr/bin/tail -1")
	if err != nil {
		log.Fatal("Remote 'df' command failed: "+output, err)
	}
	err = check_remote_lvm(output)
	if err != nil {
		log.Fatal(err)
	}
	lvmdevice := strings.Fields(output)[0]
	volgroup, volume := get_vg_and_lv(lvmdevice)
	size := strings.Fields(output)[1]
	size_curr, err := strconv.Atoi(strings.TrimSuffix(size, "G"))
	if err != nil {
		log.Fatal("Could not convert to int: ", size)
	}
	mountpoint := strings.Fields(output)[5]
	// check mountpoint equals filesystem
	if mountpoint != *FilesystemPtr {
		log.Fatal("Remote filesystem is not a mountpoint:\n == filesystem=" + *FilesystemPtr + "\n == mountpoint=" + mountpoint)
	}
	// calc how much disk space to add
	var size_needed int = *SizePtr - int(size_curr)
	if size_needed < 1 {
		log.Fatal("No resize needed: ", *SizePtr, size_curr)
	}
	fmt.Println("sizes: ", size, size_curr, size_needed)

	// get PRE scsi devices
	pre := strings.Fields(get_scsi_devices(client))
	fmt.Println("pre: ", pre)

	if *DryRunPtr {
		fmt.Print("Dry-Run detected. Exiting before making changes to remote server")
		os.Exit(0)
	}

	err = ews_add_disk(*HostPtr, token, size_needed)
	if err != nil {
		log.Fatal(err)
	}

	// get POST scsi devices
	post := strings.Fields(get_scsi_devices(client))
	fmt.Println("post: ", post)

	newdevice := get_single_new_device(pre, post)
	fmt.Println("newdevice: ", newdevice)

	var a [4]string
	a[0] = fmt.Sprintf("/usr/sbin/pvcreate %s", newdevice)
	a[1] = fmt.Sprintf("/usr/sbin/vgextend %s %s", volgroup, newdevice)
	a[2] = fmt.Sprintf("/usr/sbin/lvresize --size %dG --resizefs /dev/%s/%s", *SizePtr, volgroup, volume)

	for _, c := range a {
		output, err = remote_exec(client, c)
		if err != nil {
			log.Fatal("Remote Command failed: "+output, err)
		}
	}
}
