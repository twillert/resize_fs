package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

var HostPtr, PortPtr, FilesystemPtr *string
var SizePtr *int
var DryRunPtr *bool

func remote_exec(c *ssh.Client, cmd string) (bool, string) {
	// fmt.Println("inside create_session")
	s, err := c.NewSession()
	if err != nil {
		log.Fatal("Failed to create session: ", err)
	}
	defer s.Close()

	if out, err := s.CombinedOutput(cmd); err != nil {
		return false, string(out)
	} else {
		return true, string(out)
	}
}

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

func get_scsi_devices(c *ssh.Client) string {
	res, output := remote_exec(c, "/usr/bin/lsscsi | grep -E -o '/dev/[[:alnum:]]+'")
	if !res {
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

	var user string = os.Getenv("USER")
	var pw string = os.Getenv("pw")
	var token string = os.Getenv("token")

	if user == "" {
		log.Fatal("'USER' environment variable not set")
	}
	if pw == "" {
		log.Fatal("'pw' environment variable not set")
	}
	if token == "" {
		log.Fatal("'token' environment variable not set")
	}

	get_args()

	id := get_server_id(*HostPtr, token)
	fmt.Println(id)
	os.Exit(99)

	config := &ssh.ClientConfig{
		User: user,
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

	commands := []string{
		"file /usr/sbin/pvcreate",
		"test -x /usr/sbin/pvs",
		"test -x /usr/sbin/lvresize",
		"test -x /usr/sbin/lvs",
		"test -x /usr/bin/df",
		"test -x /usr/bin/lsscsi",
		"test -x /usr/bin/tail",
	}
	for _, s := range commands {
		res, output := remote_exec(client, s)
		if !res {
			log.Fatal("Remote requirement not met!\n == Command: " + s + "\n == Output: \n" + output)
		}
	}

	// check filesystem and get device+size for remote filesystem
	// use "df -BG" to enforce consistent GB size format
	res, output := remote_exec(client, "df -BG "+*FilesystemPtr+"| tail -1")
	if !res {
		log.Fatal("Remote 'df' command failed: " + output)
	}
	lvmdevice := strings.Fields(output)[0]
	volgroup, volume := get_vg_and_lv(lvmdevice)
	size := strings.Fields(output)[1]
	size_curr, err := strconv.Atoi(strings.TrimSuffix(size, "G"))
	if err != nil {
		log.Fatal("Could not convert to int: ", size)
	}
	mountpoint := strings.Fields(output)[5]
	// check mountpoint equals filesystemclient
	if mountpoint != *FilesystemPtr {
		log.Fatal("Remote filesystem is not a mountpoint:\n == filesystem=" + *FilesystemPtr + "\n == mountpoint=" + mountpoint)
	}
	// calc how much disk space to add
	var size_needed int = *SizePtr - int(size_curr)
	if size_needed < 1 {
		log.Fatal("No resize needed: ", *SizePtr, size_curr)
	}
	fmt.Println("sizes: ", size, size_curr, size_needed)

	// get server id
	var cmdtext string = "curl -X GET  -H 'Content-Type: application/json' --silent -H 'X-Token: " + token + "' https://api.ews.eos.lcl/api/v1/server | jq -r '.[] | select(.name==\"" + *HostPtr + "\") | .id'"
	o, err := exec.Command("bash", "-c", cmdtext).Output()
	if err != nil {
		log.Fatal(err)
	}
	var serverid string = strings.TrimSuffix(string(o), "\n")
	fmt.Printf("The server id is %s\n", serverid)

	// get PRE scsi devices
	pre := strings.Fields(get_scsi_devices(client))
	fmt.Println("pre: ", pre)

	if *DryRunPtr {
		fmt.Print("Dry-Run detected. Exiting before making changes to remote server")
		os.Exit(0)
	}
	// add disk
	cmdtext = "curl -X POST -H 'Content-Type: application/json' --silent -H 'X-Token: " + token + "' https://api.ews.eos.lcl/api/v1/server/" + serverid + "/disk -d '{ \"disks\": [ { \"disksize\": " + strconv.Itoa(size_needed) + " } ] }'"
	o, err = exec.Command("bash", "-c", cmdtext).Output()
	if err != nil {
		log.Fatal(err)
	}

	// get POST scsi devices
	post := strings.Fields(get_scsi_devices(client))
	fmt.Println("post: ", post)

	newdevice := get_single_new_device(pre, post)
	fmt.Println("newdevice: ", newdevice)

	var a [4]string
	a[0] = fmt.Sprintf("pvcreate %s", newdevice)
	a[1] = fmt.Sprintf("vgextend %s %s", volgroup, newdevice)
	a[2] = fmt.Sprintf("lvresize --size %dG --resizefs /dev/%s/%s", *SizePtr, volgroup, volume)

	for _, c := range a {
		res, output = remote_exec(client, c)
		if !res {
			log.Fatal("Remote Command failed: ", output)
		}
	}
}
