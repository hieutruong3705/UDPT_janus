package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func autoRegister(myIP, myPort, registryURL string) {
	data := map[string]string{"ip": myIP, "port": myPort}
	jsonData, _ := json.Marshal(data)

	for {
		resp, err := http.Post(registryURL, "application/json", bytes.NewBuffer(jsonData))
		if resp != nil {
			resp.Body.Close()
		}

		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			fmt.Println("[Product-Service] Registry heartbeat sent")
		} else {
			fmt.Println("[Product-Service] Waiting for Registry...")
		}

		time.Sleep(3 * time.Second)
	}
}
