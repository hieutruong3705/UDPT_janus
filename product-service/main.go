package main

import (
	"encoding/json"
	"net/http"
	"os" // Thêm thư viện os để đọc biến môi trường
)

func products(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service": "product-service",
		"products": []string{ // Đổi tên key thành products cho hợp lý
			"Cup",
			"Bottle",
			"Glass",
		},
	})
}

func main() {
	// 1. Đọc cấu hình từ Docker (Nếu chạy local thì dùng giá trị mặc định)
	registryURL := os.Getenv("REGISTRY_URL")
	if registryURL == "" {
		registryURL = "http://127.0.0.1:8080/api/register"
	}

	myHost := os.Getenv("SERVICE_HOST")
	if myHost == "" {
		myHost = "127.0.0.1"
	}

	myPort := os.Getenv("SERVICE_PORT")
	if myPort == "" {
		myPort = "9002" // Cổng 9002 của bạn
	}

	// 2. Gọi hàm tự động báo danh chạy ngầm (liên kết với discovery.go)
	go autoRegister(myHost, myPort, registryURL)

	// 3. Khởi chạy HTTP Server
	http.HandleFunc("/", products)

	println("Product Service đang chạy tại cổng :" + myPort)

	// Truyền biến myPort vào đây thay vì viết cứng ":9002"
	http.ListenAndServe(":"+myPort, nil)
}
