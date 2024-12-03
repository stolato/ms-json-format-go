# FROM golang:1.23 AS build-stage

FROM golang:1.23

# Set destination for COPY
WORKDIR /app

COPY . .

ENV GOPROXY=https://goproxy.cn

RUN go mod download

RUN env GOOS=linux GOARCH=arm go build -o main cmd/main.go

# # # Deploy the application binary into a lean image
# FROM gcr.io/distroless/base-debian11 AS build-release-stage

# WORKDIR /

# COPY --from=build-stage /app/main /main

# USER nonroot:nonroot

EXPOSE 80

CMD ["./main"]
