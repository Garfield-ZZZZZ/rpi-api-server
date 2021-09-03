package plugins

import (
	"fmt"
	"garfield/rpi-api-server/utils"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const cpuTempMetricName = "node_cpu_temperature"

type RpiTemperatureGauge struct {
	cpuTempFile string
	logger      *log.Logger
}

func (r *RpiTemperatureGauge) Start() {
	r.logger = utils.GetLogger("RpiTemperatureGauge")

	r.cpuTempFile = utils.GetEnvVarString("CPU_TEMP_FILE", "/sys/class/thermal/thermal_zone0/temp")
	r.logger.Printf("CPU_TEMP_FILE: %q", r.cpuTempFile)

	r.logger.Printf("registering Rpi CPU Temperature Gauge as %s", cpuTempMetricName)
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: cpuTempMetricName,
		Help: fmt.Sprintf("CPU temperature readout from %s", r.cpuTempFile),
	}, func() float64 {
		var ret, err = r.readCpuTemp()
		if err != nil {
			r.logger.Printf("Got exception while reading CPU temerature: %s", err)
			return 0
		}
		return ret
	})
}

func (r *RpiTemperatureGauge) HandleDebugPage(rw http.ResponseWriter, req *http.Request) {
	var cpuTemp, cpuErr = r.readCpuTemp()
	rw.WriteHeader(http.StatusOK)
	io.WriteString(rw, fmt.Sprintf(`cpuTempPath: %s
cpuTempRawValue: %f
cpuTempErr: %s
`, r.cpuTempFile, cpuTemp, cpuErr))
}

func (r *RpiTemperatureGauge) readCpuTemp() (float64, error) {
	var buf, err = ioutil.ReadFile(r.cpuTempFile)
	if err != nil {
		return -1, err
	}
	var s = string(buf)
	if len(s) <= 1 {
		return -2, fmt.Errorf("unexpected readout: %q", s)
	}
	i, err := strconv.ParseInt(s[:len(s)-1], 10, 64)
	if err != nil {
		return -3, err
	}
	return float64(i) / 1000, nil
}
