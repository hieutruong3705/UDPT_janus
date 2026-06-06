FROM golang:alpine
WORKDIR /app
COPY . .
RUN go build -o server .
EXPOSE 9001
CMD ["./server"]