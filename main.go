package main

import (
	"fmt"
	"garfield/rpi-api-server/plugins"
	"garfield/rpi-api-server/utils"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Plugin interface {
	Start()
	HandleDebugPage(rw http.ResponseWriter, req *http.Request)
}

func main() {
	var logger = utils.GetLogger("main")
	var listenAddr = utils.GetEnvVarString("LISTEN_ADDR", ":9099")
	logger.Printf("LISTEN_ADDR: %q", listenAddr)

	var enabledPlugins []Plugin
	enabledPlugins = append(enabledPlugins, &plugins.RpiTemperatureGauge{})
	enabledPlugins = append(enabledPlugins, &plugins.WhoIsAtHome{})

	var pluginMap = map[string]Plugin{}

	var sb strings.Builder
	for _, p := range enabledPlugins {
		var t = reflect.TypeOf(p)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		var name = t.Name()
		var path = fmt.Sprintf("/?plugin=%s", name)
		sb.WriteString(fmt.Sprintf("<a href=\"%s\">%s</a><br>", path, name))
		pluginMap[name] = p
		logger.Printf("starting %s", name)
		p.Start()
	}
	var mainpage = sb.String()
	utils.HttpMethodMux{
		GetHandler: func(rw http.ResponseWriter, req *http.Request) {
			var queries = req.URL.Query()
			var pluginName = queries.Get("plugin")
			if plugin, exists := pluginMap[pluginName]; exists {
				logger.Printf("got debugging page request for %s", pluginName)
				plugin.HandleDebugPage(rw, req)
			} else {
				logger.Println("got request for main page")
				rw.WriteHeader(http.StatusOK)
				io.WriteString(rw, mainpage)
			}
		},
	}.Handle("/")
	http.Handle("/metrics", promhttp.Handler())

	logger.Println("starting http server")
	logger.Fatal(http.ListenAndServe(listenAddr, nil))
}
