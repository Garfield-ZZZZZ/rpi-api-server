package main

import (
	"fmt"
	"garfield/rpi-api-server/plugins"
	"garfield/rpi-api-server/utils"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	var logger = utils.GetLogger("main")
	var listenAddr = utils.GetEnvVarString("LISTEN_ADDR", ":9099")
	var pluginName = utils.GetEnvVarString("PLUGIN_NAME", "")
	logger.Printf("LISTEN_ADDR: %q", listenAddr)
	logger.Printf("PLUGIN_NAME: %q", pluginName)

	var pluginMap = map[string]plugins.Plugin{
		"temperature":    &plugins.RpiTemperatureGauge{},
		"ishome":         &plugins.WhoIsAtHome{},
		"networkmonitor": &plugins.NetworkMonitor{},
	}

	var plugin, exists = pluginMap[pluginName]
	if !exists {
		logger.Printf("plugin %q not found", pluginName)
		for name := range pluginMap {
			logger.Printf("available plugin: %s", name)
		}

		panic("invalid plugin name")
	}

	logger.Printf("starting %s", pluginName)
	plugin.Start()
	var path = fmt.Sprintf("/%s", pluginName)
	logger.Printf("registering handler at %s", path)
	http.Handle(path, plugin)

	http.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		http.Redirect(rw, req, path, http.StatusFound)
	})
	http.Handle("/metrics", promhttp.Handler())

	logger.Println("starting http server")
	logger.Fatal(http.ListenAndServe(listenAddr, nil))
}
