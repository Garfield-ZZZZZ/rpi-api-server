package plugins

import (
	"fmt"
	"encoding/json"
	"garfield/rpi-api-server/utils"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const networkAvailabilityMetricName = "network_availability"
const networkAvailabilityTargetLabel = "target"

var networkAvailabilityLables = []string{networkAvailabilityTargetLabel}

type NetworkAvailability struct {
	logger                   *log.Logger
	targets                  map[string]Target
	ticker                   *time.Ticker
	gaugeVec                 *prometheus.GaugeVec
	lastTick                 time.Time
	refreshIntervalInSeconds int
	proxyUrlStr              string
	httpClientWithProxy      *http.Client
}

type Target struct {
	Url       string `json:"url"`
	NeedProxy bool   `json:"needProxy"`
}

func (n *NetworkAvailability) Start() {
	n.logger = utils.GetLogger("NetworkAvailabilityGauge")
	filePath := "targets.json"
	jsonBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		n.logger.Printf(err.Error())
		return
	}
	n.logger.Printf(string(jsonBytes))
	err = json.Unmarshal(jsonBytes, &n.targets)
	if err != nil {
		n.logger.Printf(err.Error())
		return
	}
	for name, target := range n.targets {
		n.logger.Printf("got target %s at %s with proxy %t", name, target.Url, target.NeedProxy)
	}

	n.refreshIntervalInSeconds = utils.GetEnvVarInt("RefreshIntervalInSeconds", 300)
	n.proxyUrlStr = utils.GetEnvVarString("ProxyUrl", "://invalidURL")
	proxyUrl, err := url.Parse(n.proxyUrlStr)
	if err != nil {
		n.logger.Printf("failed to parse proxy url %q: %s", n.proxyUrlStr, err)
		n.httpClientWithProxy = nil
	} else {
		n.httpClientWithProxy = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)}}
	}

	n.logger.Printf("registering network availability gauge as %s", networkAvailabilityMetricName)
	n.logger.Printf("refresh interval is %d seconds", n.refreshIntervalInSeconds)
	n.logger.Printf("proxy url is %q", n.proxyUrlStr)

	n.gaugeVec = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: networkAvailabilityMetricName,
		Help: "check network availability by http",
	}, networkAvailabilityLables)

	n.ticker = time.NewTicker(time.Duration(n.refreshIntervalInSeconds) * time.Second)
	go n.ticking()
}

func (n *NetworkAvailability) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var avail = n.checkAvailability()
	io.WriteString(rw, fmt.Sprintf("refreshIntervalInSeconds: %d\n", n.refreshIntervalInSeconds))
	io.WriteString(rw, fmt.Sprintf("proxyUrl: %q\n", n.proxyUrlStr))
	for name := range avail {
		io.WriteString(rw, fmt.Sprintf("%s(%q) is at %t\n", name, n.targets[name].Url, avail[name] == 1))
	}
}

func (n *NetworkAvailability) ticking() {
	var labels = map[string]string{networkAvailabilityTargetLabel: ""}
	n.lastTick = time.Now()
	for ; true; n.lastTick = <-n.ticker.C {
		n.logger.Printf("tick on %s started", n.lastTick)
		var avail = n.checkAvailability()
		for name := range avail {
			labels[networkAvailabilityTargetLabel] = name
			n.gaugeVec.With(labels).Set(avail[name])
		}
		n.logger.Printf("tick on %s completed", n.lastTick)
	}
}

func (n *NetworkAvailability) checkAvailability() map[string]float64 {
	var ret = map[string]float64{}
	for name, target := range n.targets {
		n.logger.Printf("checking %s at %s with proxy %t", name, target.Url, target.NeedProxy)
		var httpClient = http.DefaultClient
		if target.NeedProxy && n.httpClientWithProxy != nil {
			httpClient = n.httpClientWithProxy
		}
		resp, err := httpClient.Get(target.Url)
		if err != nil {
			n.logger.Printf("got error while checking %s: %s", name, err)
			ret[name] = 0
		} else {
			n.logger.Printf("got %d from %s", resp.StatusCode, name)
			ret[name] = 1
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	return ret
}
