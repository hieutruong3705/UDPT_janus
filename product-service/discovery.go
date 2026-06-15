package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func autoRegister(myIP, myPort, registryURLs string) {
	data := map[string]string{"ip": myIP, "port": myPort}
	jsonData, _ := json.Marshal(data)
	urls := parseRegistryURLs(registryURLs)

	for {
		for _, registryURL := range urls {
			resp, err := http.Post(registryURL, "application/json", bytes.NewBuffer(jsonData))
			if resp != nil {
				resp.Body.Close()
			}

			if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
				fmt.Println("[Product-Service] Registry heartbeat sent to " + registryURL)
			} else {
				fmt.Println("[Product-Service] Waiting for Registry " + registryURL + "...")
			}
		}

		time.Sleep(3 * time.Second)
	}
}

func parseRegistryURLs(raw string) []string {
	var urls []string
	for _, item := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';'
	}) {
		item = strings.TrimSpace(item)
		if item != "" {
			urls = append(urls, item)
		}
	}
	return urls
}
