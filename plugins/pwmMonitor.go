package plugins

import (
	"fmt"
	"garfield/rpi-api-server/utils"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const pwmExportedMetricName = "node_pwm_exported"
const pwmEnabledMetricName = "node_pwm_enabled"
const pwmPeroidMetricName = "node_pwm_peroid"
const pwmDutyCycleMetricName = "node_pwm_duty_cycle"

type PwmGauge struct {
	logger        *log.Logger
	pwmChipFolder string
}

func (p *PwmGauge) Start() {
	p.logger = utils.GetLogger("PwmGauge")

	p.pwmChipFolder = utils.GetEnvVarString("PWM_CHIP_FOLDER", "/sys/class/pwm/pwmchip0/pwm0")
	p.logger.Printf("PWM_CHIP_FOLDER: %q", p.pwmChipFolder)

	p.logger.Printf("registering exported Gauge as %s", pwmExportedMetricName)
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: pwmExportedMetricName,
		Help: fmt.Sprintf("If %q is exported", p.pwmChipFolder),
	}, func() float64 {
		if p.getExported() {
			return 1
		} else {
			return 0
		}
	})

	p.logger.Printf("registering enabled Gauge as %s", pwmEnabledMetricName)
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: pwmEnabledMetricName,
		Help: fmt.Sprintf("If %q is enabled", p.pwmChipFolder),
	}, func() float64 {
		if p.getEnabled() {
			return 1
		} else {
			return 0
		}
	})

	p.logger.Printf("registering peroid Gauge as %s", pwmPeroidMetricName)
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: pwmPeroidMetricName,
		Help: fmt.Sprintf("peroid setting of %q", p.pwmChipFolder),
	}, func() float64 {
		return float64(p.getPeroid())
	})

	p.logger.Printf("registering duty cycle Gauge as %s", pwmDutyCycleMetricName)
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: pwmDutyCycleMetricName,
		Help: fmt.Sprintf("duty cycle setting of %q", p.pwmChipFolder),
	}, func() float64 {
		return float64(p.getDutyCycle())
	})
}

func (p *PwmGauge) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusOK)
	io.WriteString(rw, fmt.Sprintf(`exported: %t
enabled: %t
peroid: %d
duty cycle: %d
`, p.getExported(), p.getEnabled(), p.getPeroid(), p.getDutyCycle()))
}

func (p *PwmGauge) getExported() bool {
	_, err := os.Stat(p.pwmChipFolder)
	return !os.IsNotExist(err)
}

func (p *PwmGauge) getEnabled() bool {
	if !p.getExported() {
		return false
	}
	var num = p.readFileAsInt64(p.pwmChipFolder + "/enable")
	return num == 1
}

func (p *PwmGauge) getPeroid() int64 {
	if !p.getExported() {
		return 0
	}
	var num = p.readFileAsInt64(p.pwmChipFolder + "/period")
	return num
}

func (p *PwmGauge) getDutyCycle() int64 {
	if !p.getExported() {
		return 0
	}
	var num = p.readFileAsInt64(p.pwmChipFolder + "/duty_cycle")
	return num
}

func (p *PwmGauge) readFileAsInt64(path string) int64 {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		p.logger.Printf("Error reading file: %v\n", err)
		return 0
	}
    var str = strings.TrimRight(string(content), "\n")
	num, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		p.logger.Printf("Error converting file content to int64: %v\n", err)
		return 0
	}
	return num
}
