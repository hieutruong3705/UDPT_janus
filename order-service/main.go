package main

import (
	"encoding/json"
	"net/http"
	"os" // Thêm thư viện này để đọc cấu hình Docker
)

func orders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service": "order-service",
		"orders": []string{ // Đổi key thành orders cho hợp lý
			"Order_1",
			"Order_2",
			"Order_3",
		},
	})
}

func main() {
	// 1. Đọc cấu hình từ biến môi trường Docker
	registryURL := os.Getenv("REGISTRY_URL")
	if registryURL == "" {
		registryURL = "http://127.0.0.1:8880/api/register"
	}

	myHost := os.Getenv("SERVICE_HOST")
	if myHost == "" {
		myHost = "127.0.0.1"
	}

	myPort := os.Getenv("SERVICE_PORT")
	if myPort == "" {
		myPort = "9003"
	}

	// 2. GỌI HÀM BÁO DANH CHẠY NGẦM (Dòng quan trọng nhất bị thiếu)
	go autoRegister(myHost, myPort, registryURL)

	// 3. Khởi chạy HTTP Server
	http.HandleFunc("/", orders)

	println("Order Service đang chạy tại cổng :" + myPort)

	// Truyền biến myPort vào đây thay vì viết cứng
	http.ListenAndServe(":"+myPort, nil)
}
