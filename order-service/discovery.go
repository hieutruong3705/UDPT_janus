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
		if err == nil && resp.StatusCode == http.StatusOK {
			fmt.Println("✅ [Order-Service] Đã báo danh thành công với Registry!")
			resp.Body.Close()
			break
		}
		fmt.Println("⏳ [Order-Service] Đang tìm Registry để báo danh...")
		time.Sleep(3 * time.Second)
	}
}
