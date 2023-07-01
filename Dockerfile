FROM golang:1.16 as builder

RUN mkdir /app
WORKDIR /app
COPY . .
RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod download

# Build
RUN CGO_ENABLED=1 GOOS=linux GOARCH=arm go build -a -o the_binary main.go

FROM ubuntu:22.04
RUN apt-get update && apt-get install -y \
    iptables \
&& rm -rf /var/lib/apt/lists/*
WORKDIR /
COPY --from=builder /app/the_binary .

ENTRYPOINT ["/the_binary"]
