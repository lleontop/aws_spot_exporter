package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/lleontop/aws_audit_exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	listenAddress = kingpin.Flag("web.listen-address", "Port to listen on").Default(":9190").String()
	metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
)

func init() {
	prometheus.MustRegister(version.NewCollector("aws_spot_exporter"))
}

func main() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("aws_spot_market_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting aws_spot_market_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	exporter, err := collector.NewExporter()
	if err != nil {
		log.Fatalln(err)
	}
	if exporter == nil {
		log.Fatalln("Exporter is null!!")
	}
	prometheus.MustRegister(exporter)

	go serveMetrics()

	exitChannel := make(chan os.Signal)
	signal.Notify(exitChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	exitSignal := <-exitChannel
	log.Infof("Caught %s signal, exiting", exitSignal)
}

func serveMetrics() {
	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>AWS spot market Exporter</title></head>
			<body>
			<h1>AWS spot market Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`))
	})
	log.Infoln("Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
