FROM golang:1.16 as builder

RUN mkdir /app
WORKDIR /app
COPY . .
RUN go mod download

# Build
RUN CGO_ENABLED=1 GOOS=linux GOARCH=arm go build -a -o the_binary main.go

FROM alpine:3.14.1
# install gcompat per https://wiki.alpinelinux.org/wiki/Running_glibc_programs
RUN apk add --no-cache iptables gcompat
WORKDIR /
COPY --from=builder /app/the_binary .

ENTRYPOINT ["/the_binary"]
