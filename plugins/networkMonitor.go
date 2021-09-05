package plugins

import (
	"fmt"
	"garfield/rpi-api-server/utils"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// requires libpcap-dev
const hostLabel = "host"
const directionLable = "direction"
const promNamespace = "node_traffic"

type NetworkMonitor struct {
	targetInterface string
	broadcastIPNet  *net.IPNet
	selfIp          net.IP
	packetBuffer    chan gopacket.Packet
	trafficCounter  *prometheus.CounterVec
	logger          *log.Logger

	totalPacketCnt          int
	totalBytes              int
	totalTrafficCounterChan chan int
}

func (n *NetworkMonitor) Start() {
	n.logger = utils.GetLogger("NetworkMonitor")
	n.targetInterface = utils.GetEnvVarString("NETWORK_MONITOR_INTERFACE", "eth0")
	var bufferSize = utils.GetEnvVarInt("NETWORK_PACKET_BUFFER_SIZE", 100)
	var workerCount = utils.GetEnvVarInt("WORKER_COUNT", 1)
	n.logger.Printf("NETWORK_MONITOR_INTERFACE: %s", n.targetInterface)
	n.logger.Printf("NETWORK_PACKET_BUFFER_SIZE: %d", bufferSize)
	n.logger.Printf("WORKER_COUNT: %d", workerCount)

	n.totalTrafficCounterChan = make(chan int)
	n.packetBuffer = make(chan gopacket.Packet, bufferSize)
	n.trafficCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: promNamespace,
		Name:      "total_bytes",
	}, []string{"host", "direction"})
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: promNamespace,
		Name:      "monitor_buffer_size",
	}, func() float64 {
		return float64(len(n.packetBuffer))
	})

	n.logger.Printf("querying interfaces")
	ifaces, err := net.Interfaces()
	if err != nil {
		n.logger.Panicf("failed to get network interfaces: %s", err)
	}
	for _, i := range ifaces {
		if i.Name != n.targetInterface {
			continue
		}
		n.logger.Printf("found interface %s", n.targetInterface)
		addrs, err := i.Addrs()
		if err == nil {
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				ip = ip.To4()
				if ip != nil {
					n.selfIp = ip
					break
				}
			}
		}
	}
	if n.selfIp == nil {
		n.logger.Panicf("failed to get self ip")
	} else {
		n.logger.Printf("self ip is set to %s", n.selfIp)
	}

	_, ipv4Net, err := net.ParseCIDR("224.0.0.0/4")
	if err != nil {
		n.logger.Panicf("failed to parse boardcast cidr: %s", err)
	}
	n.broadcastIPNet = ipv4Net

	for i := 0; i < workerCount; i++ {
		go n.rerunWhatever(fmt.Sprintf("worker %d", i), n.processChan)
	}
	go n.rerunWhatever("totalTrafficCounter", n.collectDebugData)

	n.logger.Printf("starting capture go routine")
	go n.startCapture()
}

func (n *NetworkMonitor) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusOK)
	io.WriteString(rw, fmt.Sprintf(`self ip: %s
current buffer length: %d
total packet count: %d
total traffic in MB: %d
`, n.selfIp.String(), len(n.packetBuffer), n.totalPacketCnt, n.totalBytes/1024/1024))
}

func (n *NetworkMonitor) startCapture() {
	defer func() {
		if x := recover(); x != nil {
			n.logger.Printf("got panic %v", x)
		}
		n.logger.Fatalf("capture function exited, quiting everything")
	}()
	for {
		n.logger.Printf("starting capture packets")
		n.totalPacketCnt = 0
		if handle, err := pcap.OpenLive(n.targetInterface, 100, false, pcap.BlockForever); err != nil {
			n.logger.Panicf("failed to open interface: %s", err)
		} else {
			packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
			for packet := range packetSource.Packets() {
				n.packetBuffer <- packet
			}
		}
	}
}

func (n *NetworkMonitor) processChan() {
	var inboundLables = map[string]string{hostLabel: "", directionLable: "inbound"}
	var outboundLables = map[string]string{hostLabel: "", directionLable: "outbound"}

	for {
		var packet = <-n.packetBuffer
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer != nil {
			ip, _ := ipLayer.(*layers.IPv4)
			var inbound = 0
			var outbound = 0
			var host = ""
			if ip.SrcIP.Equal(n.selfIp) {
				outbound = packet.Metadata().Length
				n.totalTrafficCounterChan <- outbound
				host = ip.DstIP.String()
			} else if ip.DstIP.Equal(n.selfIp) {
				inbound = packet.Metadata().Length
				n.totalTrafficCounterChan <- inbound
				host = ip.SrcIP.String()
			} else if n.broadcastIPNet.Contains(ip.DstIP) {
				inbound = packet.Metadata().Length
				n.totalTrafficCounterChan <- inbound
				host = ip.SrcIP.String()
			} else {
				host = fmt.Sprintf("%s => %s", ip.SrcIP.String(), ip.DstIP.String())
			}
			inboundLables[hostLabel] = host
			outboundLables[hostLabel] = host
			n.trafficCounter.With(inboundLables).Add(float64(inbound))
			n.trafficCounter.With(outboundLables).Add(float64(outbound))
		}
	}
}

func (n *NetworkMonitor) collectDebugData() {
	for {
		var size = <-n.totalTrafficCounterChan
		n.totalPacketCnt++
		n.totalBytes += size
	}
}

func (n *NetworkMonitor) rerunWhatever(name string, target func()) {
	for {
		n.logger.Printf("starting to run %s", name)
		func() {
			defer func() {
				n.logger.Printf("function %s completed", name)
				var p = recover()
				if p != nil {
					n.logger.Printf("got panic: %v", p)
				}
			}()
			target()
		}()
	}
}
