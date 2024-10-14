FROM golang:1.21

# Set destination for COPY
WORKDIR /app

COPY . .

ENV GOPROXY=https://goproxy.cn

RUN go mod download

RUN go build -o main cmd/main.go

EXPOSE 8000

CMD ["./main"]
