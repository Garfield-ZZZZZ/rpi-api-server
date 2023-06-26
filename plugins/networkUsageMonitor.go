// list iptables rules and extract packet and byte usage of devices
// iptalbes rules should have comment like "device_name: some-device"

package plugins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"garfield/rpi-api-server/utils"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type NetworkUsageMonitor struct {
	chainName                string
	commentKey               string
	command                  string
	counterVec               *prometheus.CounterVec
	lastKnownValue           *map[string]DeviceUsage
	lastStdOut               string
	logger                   *log.Logger
	refreshIntervalInSeconds int
	ticker                   *time.Ticker
}

type DeviceUsage struct {
	Packets float64
	Bytes   float64
}

const networkUsageMonitorMetricName = "network_usage_monitor"
const deviceNameLabel = "device_name"
const metricTypeLabel = "metric_type"
const metricTypePackets = "packets"
const metricTypeBytes = "bytes"

var networkUsageMonitorLables = []string{deviceNameLabel, metricTypeLabel}

func (n *NetworkUsageMonitor) Start() {
	n.logger = utils.GetLogger("NetworkUsageMonitor")
	n.chainName = utils.GetEnvVarString("CHAIN_NAME", "NETWORK-FILTER")
	n.commentKey = utils.GetEnvVarString("COMMENT_KEY", "device_name")
	n.command = utils.GetEnvVarString("COMMAND", fmt.Sprintf("iptables -nxvL %s | grep %s | tr -s ' ' | cut -d ' ' -f 2,3,13", n.chainName, n.commentKey))
	n.refreshIntervalInSeconds = utils.GetEnvVarInt("RefreshIntervalInSeconds", 300)
	n.logger.Printf("CHAIN_NAME: %q", n.chainName)
	n.logger.Printf("COMMENT_KEY: %q", n.commentKey)
	n.logger.Printf("COMMAND: %q", n.command)
	n.logger.Printf("refresh interval is %d seconds", n.refreshIntervalInSeconds)

	n.lastKnownValue = &map[string]DeviceUsage{}

	n.counterVec = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: networkUsageMonitorMetricName,
		Help: "extract packet and byte usage of devices from iptalbes rules",
	}, networkUsageMonitorLables)

	n.ticker = time.NewTicker(time.Duration(n.refreshIntervalInSeconds) * time.Second)
	go n.ticking()
}

func (n *NetworkUsageMonitor) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusOK)
	io.WriteString(rw, fmt.Sprintf("command is : %q\n", n.command))
	var lastKnownValueJson, err = json.Marshal(n.lastKnownValue)
	if err != nil {
		io.WriteString(rw, fmt.Sprintf(`marshal error: %s`, err.Error()))
	}
	io.WriteString(rw, fmt.Sprintf("last known: %s\n", lastKnownValueJson))

	var usage = n.GetCurrentUsage()
	usageJson, err := json.Marshal(usage)
	if err != nil {
		io.WriteString(rw, fmt.Sprintf(`marshal error: %s`, err.Error()))
	}
	io.WriteString(rw, fmt.Sprintf("std out: %s\n", n.lastStdOut))
	io.WriteString(rw, fmt.Sprintf("current usage: %s\n", usageJson))

	var diff = n.GetIncremental(n.lastKnownValue, usage)
	diffJson, err := json.Marshal(diff)
	if err != nil {
		io.WriteString(rw, fmt.Sprintf(`marshal error: %s`, err.Error()))
	}
	io.WriteString(rw, fmt.Sprintf("incremental usage: %s\n", diffJson))
}

func (n *NetworkUsageMonitor) ticking() {
	var labels = map[string]string{deviceNameLabel: "", metricTypeLabel: ""}
	var lastTick = time.Now()
	for ; true; lastTick = <-n.ticker.C {
		n.logger.Printf("tick on %s started", lastTick)
		n.logger.Printf("retriving current usage")
		var currentUsage = n.GetCurrentUsage()
		n.logger.Printf("calculating incremental")
		var incremental = n.GetIncremental(n.lastKnownValue, currentUsage)
		n.logger.Printf("saving current usage as last known")
		n.lastKnownValue = currentUsage

		for name := range *incremental {
			labels[deviceNameLabel] = name
			labels[metricTypeLabel] = metricTypePackets
			n.counterVec.With(labels).Add((*incremental)[name].Packets)
			labels[metricTypeLabel] = metricTypeBytes
			n.counterVec.With(labels).Add((*incremental)[name].Bytes)
		}
		n.logger.Printf("tick on %s completed", lastTick)
	}
}

func (n *NetworkUsageMonitor) GetCurrentUsage() *map[string]DeviceUsage {
	cmd := exec.Command("bash", "-c", n.command)
	var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    if err := cmd.Run(); err != nil {
		n.logger.Printf(fmt.Sprintf("got error while running command: %q\n", err.Error()))
        return nil
    }
	if stderr.Len() > 0 {
		n.logger.Printf(fmt.Sprintf("got stderr: %q\n", stderr.String()))
		return nil
	}
	n.lastStdOut = stdout.String()

	lines := bytes.Split(stdout.Bytes(), []byte("\n"))
	usage := make(map[string]DeviceUsage)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var cols = bytes.Split(line, []byte(" "))
		if len(cols) < 3 {
			n.logger.Printf(fmt.Sprintf("invalid line: %q\n", line))
			continue
		}
		var deviceName = string(cols[2])
		var packetCount, packetErr = strconv.ParseFloat(string(cols[0]), 64)
		if packetErr != nil {
			n.logger.Printf(fmt.Sprintf("invalid packet cnt: %q\n", line))
			continue
		}
		// parse cols[1] as float64
		var byteCount, byteErr = strconv.ParseFloat(string(cols[1]), 64)
		if byteErr != nil {
			n.logger.Printf(fmt.Sprintf("invalid bytes cnt: %q\n", line))
			continue
		}
		usage[deviceName] = DeviceUsage{
			Packets: packetCount,
			Bytes:   byteCount,
		}
	}

	return &usage;
}

func (n *NetworkUsageMonitor) GetIncremental(lastKnown *map[string]DeviceUsage, currentValues *map[string]DeviceUsage) *map[string]DeviceUsage {
	var ret = make(map[string]DeviceUsage)
	for name, current := range *currentValues {
		var last, ok = (*lastKnown)[name]
		if !ok {
			n.logger.Printf(fmt.Sprintf("device %q not found in last known usage", name))
			ret[name] = DeviceUsage{
				Packets: current.Packets,
				Bytes:   current.Bytes,
			}
		}
		ret[name] = DeviceUsage{
			Packets: current.Packets - last.Packets,
			Bytes:   current.Bytes - last.Bytes,
		}
	}
	return &ret
}