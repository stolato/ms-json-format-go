FROM golang:1.25-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o main cmd/main.go

FROM scratch

WORKDIR /

COPY --from=build /app/main /main
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8000
EXPOSE 9091

USER 1001

CMD ["/main"]
