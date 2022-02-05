package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

func get_server_id(host string, token string) (serverid int, err error) {

	url := "https://api.ews.eos.lcl/api/v1/server?name=" + host
	c := http.Client{Timeout: time.Duration(60) * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}

	req.Header.Add("X-Token", token)
	resp, err := c.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	fmt.Println("response Body:", string(body))

	type Server struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	}

	var s []Server
	err = json.Unmarshal(body, &s)
	if err != nil {
		return
	}

	// assume only 1 element in array
	if len(s) != 1 {
		return 0, fmt.Errorf("API search returned multiple IDs, array length=  %v", len(s))
	}

	serverid = s[0].Id
	return
}

func add_disk(serverid int, disksize int, token string) (err error) {

	url := "https://api.ews.eos.lcl/api/v1/server/" + strconv.Itoa(serverid) + "/disk"
	var postbody = []byte(`{ "disks": [ { "disksize": ` + strconv.Itoa(disksize) + `} ] }`)
	// var cmdtext string = "curl -X POST -H 'Content-Type: application/json' --silent -H 'X-Token: " + token + "' https://api.ews.eos.lcl/api/v1/server/" + serverid + "/disk -d '{ \"disks\": [ { \"disksize\": " + strconv.Itoa(size_needed) + " } ] }'"
	c := http.Client{Timeout: time.Duration(60) * time.Second}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(postbody))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("X-Token", token)

	resp, err := c.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	return

}
