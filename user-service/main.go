package main

import (
	"encoding/json"
	"net/http"
	"os" // Thêm thư viện này để đọc cấu hình Docker
)

func users(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service": "user-service",
		"users": []string{
			"Hai",
			"An",
			"Binh",
		},
	})
}

func main() {
	// 1. Đọc cấu hình từ Docker (Nếu chạy local không có biến môi trường thì dùng giá trị mặc định)
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
		myPort = "9001" // Giữ nguyên cổng 9001 hiện tại của bạn làm mặc định
	}

	// 2. Gọi hàm tự động báo danh chạy ngầm (nằm trong file discovery.go cùng thư mục)
	go autoRegister(myHost, myPort, registryURL)

	// 3. Khởi chạy HTTP Server
	http.HandleFunc("/", users)

	println("User Service đang chạy tại cổng :" + myPort)

	// Truyền biến myPort vào đây thay vì viết cứng ":9001"
	http.ListenAndServe(":"+myPort, nil)
}
