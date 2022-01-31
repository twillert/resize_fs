package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

func get_server_id(host string, token string) (int, error) {
	c := http.Client{Timeout: time.Duration(60) * time.Second}
	req, err := http.NewRequest("GET", "https://Xapi.ews.eos.lcl/api/v1/server?name="+host, nil)
	if err != nil {
		return 0, fmt.Errorf("Error1: %v", err)
		// log.Fatal("Error quering server endpoint: ", err)
	}
	req.Header.Add("X-Token", token)

	resp, err := c.Do(req)
	if err != nil {
		log.Fatal("Error getting request: ", err)

	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading body: ", err)
	}

	type Server struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	}

	var s []Server
	err = json.Unmarshal(body, &s)
	if err != nil {
		log.Fatal("Decoding json: ", err, body)
	}

	// assume only 1 element in array
	return s[0].Id, nil
}
