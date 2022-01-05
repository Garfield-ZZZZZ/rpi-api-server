package plugins

import (
	"encoding/json"
	"fmt"
	"garfield/rpi-api-server/utils"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	promGaugeName       = "family_member_ishome"
	promMemberLabel     = "name"
	promReqCounterName  = "family_api_request"
	promReqTypeLabel    = "type"
	promRespStatusLabel = "status"
)

type WhoIsAtHome struct {
	currentStatus        map[string]bool
	notification         utils.NotificationPusher
	notificationPriority int
	statusGaugeVec       *prometheus.GaugeVec
	respCounterVec       *prometheus.CounterVec
	logger               *log.Logger
}

func (w *WhoIsAtHome) Start() {
	w.logger = utils.GetLogger("WhoIsAtHome")

	w.notification = utils.GetNotificationPusher()
	w.notificationPriority = utils.GetEnvVarInt("WHOISATHOME_NOTIFICATION_PRIORITY", -1)
	w.logger.Printf("WHOISATHOME_NOTIFICATION_PRIORITY: %d", w.notificationPriority)

	w.logger.Printf("registering gauge as %s", promGaugeName)
	w.statusGaugeVec = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: promGaugeName}, []string{promMemberLabel})
	w.logger.Printf("registering request counter as %s", promReqCounterName)
	w.respCounterVec = promauto.NewCounterVec(prometheus.CounterOpts{Name: promReqCounterName}, []string{promRespStatusLabel, promReqTypeLabel})

	w.currentStatus = map[string]bool{}
	var validUsers = utils.GetEnvVarString("WHOISATHOME_USERS", "")
	w.logger.Printf("WHOISATHOME_USERS: %q", validUsers)
	for _, user := range strings.Split(validUsers, " ") {
		if user != "" {
			w.currentStatus[user] = true
		}
		w.statusGaugeVec.With(map[string]string{promMemberLabel: user}).Set(w.getGaugeStatusForIsHome(true))
	}
}
func (w *WhoIsAtHome) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	w.logger.Printf("Got new request from %q for %q using method %s", req.RemoteAddr, req.RequestURI, req.Method)
	switch req.Method {
	case http.MethodGet:
		w.HandleDebugPage(rw, req)
	case http.MethodPost:
		w.HandleUpdate(rw, req)
	default:
		w.logger.Printf("unregistered method")
		rw.WriteHeader(http.StatusBadRequest)
		io.WriteString(rw, fmt.Sprintf("invalid method: %s\n", req.Method))
	}
}

func (w *WhoIsAtHome) HandleUpdate(rw http.ResponseWriter, req *http.Request) {
	var respStatus int64 = http.StatusInternalServerError
	defer func() {
		w.respCounterVec.With(map[string]string{promRespStatusLabel: strconv.FormatInt(respStatus, 10), promReqTypeLabel: "update"}).Inc()
	}()

	// read request and validate
	var queries = req.URL.Query()
	var who = queries.Get("who")
	var isHomeStr = queries.Get("ishome")
	w.logger.Printf("got update request, who: %s, isHome: %s", who, isHomeStr)
	if who == "" || isHomeStr == "" {
		w.logger.Println("invalid request")
		respStatus = http.StatusBadRequest
		rw.WriteHeader(http.StatusBadRequest)
		io.WriteString(rw, "invalid request, needs to specify who (string) and isHome (bool)\n")
		return
	}
	var isHome, err = strconv.ParseBool(isHomeStr)
	if err != nil {
		w.logger.Println("failed to parse isHome")
		respStatus = http.StatusBadRequest
		rw.WriteHeader(http.StatusBadRequest)
		io.WriteString(rw, fmt.Sprintf("invalid valid %s for isHome, should be a bool\n", isHomeStr))
		return
	}
	var isValidUser = false
	for name := range w.currentStatus {
		if who == name {
			isValidUser = true
			break
		}
	}
	if !isValidUser {
		w.logger.Println("invalid user")
		respStatus = http.StatusBadRequest
		rw.WriteHeader(http.StatusBadRequest)
		io.WriteString(rw, "invalid user\n")
		return
	}
	w.logger.Println("updating internal status")
	w.currentStatus[who] = isHome
	// update gauge
	w.logger.Println("updating prometheus gauge")
	w.statusGaugeVec.With(map[string]string{promMemberLabel: who}).Set(w.getGaugeStatusForIsHome(isHome))
	// send message
	w.logger.Println("sending notification")
	var statusStr = "home"
	if !isHome {
		statusStr = "away"
	}
	if w.notification != nil {
		err = w.notification.Send("Home", fmt.Sprintf("%s is %s", who, statusStr), w.notificationPriority)
	}
	if err != nil {
		w.logger.Printf("failed to send notification: %s", err)
		respStatus = http.StatusInternalServerError
		rw.WriteHeader(http.StatusInternalServerError)
		io.WriteString(rw, fmt.Sprintf("failed to send notification: %s\n", err))
		return
	}
	w.logger.Println("update completed")
	respStatus = http.StatusOK
	rw.WriteHeader(http.StatusOK)
}

func (w *WhoIsAtHome) HandleDebugPage(rw http.ResponseWriter, req *http.Request) {
	var respStatus int64 = http.StatusInternalServerError
	defer func() {
		w.respCounterVec.With(map[string]string{promRespStatusLabel: strconv.FormatInt(respStatus, 10), promReqTypeLabel: "get"}).Inc()
	}()

	var bytes, err = json.MarshalIndent(w.currentStatus, "", "    ")
	if err != nil {
		respStatus = http.StatusInternalServerError
		rw.WriteHeader(http.StatusInternalServerError)
	} else {
		respStatus = http.StatusOK
		rw.WriteHeader(http.StatusOK)
		rw.Write(bytes)
	}
}

func (w *WhoIsAtHome) getGaugeStatusForIsHome(isHome bool) float64 {
	if isHome {
		return 1
	} else {
		return 0
	}
}
