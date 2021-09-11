FROM golang:1.16 as builder

# download and build libpcap
# https://github.com/google/gopacket/issues/734
# https://github.com/the-tcpdump-group/libpcap/issues/733
ARG PCAPV=1.9.1
RUN apt-get update
RUN apt-get install -y flex bison libpcap-dev
WORKDIR /tmp
RUN wget http://www.tcpdump.org/release/libpcap-$PCAPV.tar.gz
RUN tar -xf libpcap-$PCAPV.tar.gz
WORKDIR /tmp/libpcap-$PCAPV
RUN ./configure --with-pcap=linux
RUN make

RUN mkdir /app
WORKDIR /app
COPY . .

RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod download

# Build
RUN CGO_ENABLED=1 GOOS=linux GOARCH=arm go build --ldflags "-L /tmp/libpcap-$PCAPV -linkmode external -extldflags \"-static\"" -a -o the_binary main.go

FROM alpine:3.14.1
WORKDIR /
COPY --from=builder /app/the_binary .

ENTRYPOINT ["/the_binary"]
