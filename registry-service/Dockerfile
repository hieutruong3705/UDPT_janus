# File: registry-service/Dockerfile
FROM golang:1.20-alpine

WORKDIR /app

# Copy file quản lý package
COPY go.mod ./
RUN go mod download

# Copy toàn bộ mã nguồn vào container
COPY . .

# Biên dịch ứng dụng Go
RUN go build -o main .

# Mở cổng 8080 của container
EXPOSE 8080

# Chạy ứng dụng
CMD ["./main"]