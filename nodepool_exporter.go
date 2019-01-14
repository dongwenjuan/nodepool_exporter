// Copyright 2013 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
    "fmt"
    "io/ioutil"
	"net/http"
    "sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "nodepool" // For Prometheus metrics.
)

func init() {
	prometheus.MustRegister(version.NewCollector("nodepool_exporter"))
}

type Host struct {
	name    string
	port        string
}

type Exporter struct {
	scrapeHost      Host
	mutex           sync.Mutex
	client          *http.Client
	up              *prometheus.Desc
	scrapeFailures  prometheus.Counter
}

func NewExporter(host Host) *Exporter {
	return &Exporter{
		scrapeHost: host,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: nil},
		},
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Could the nodepool server be reached",
			[]string{"host"},
			nil),
		scrapeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrape_failures_total",
			Help:      "Number of errors while scraping apache.",
		}),
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.up
	e.scrapeFailures.Describe(ch)
}

func (e *Exporter) collect(ch chan<- prometheus.Metric) error {

    scrapeURI := "http://" + e.scrapeHost.name + ":" + e.scrapeHost.port + "/image-list"
	resp, err := e.client.Get(scrapeURI)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0, e.scrapeHost.name)
		return fmt.Errorf("Error scraping nodepool: %v", err)
	}
	ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 1, e.scrapeHost.name)

	data, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		if err != nil {
			data = []byte(err.Error())
		}
		return fmt.Errorf("Status %s (%d): %s", resp.Status, resp.StatusCode, data)
	}
    return nil
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()
	if err := e.collect(ch); err != nil {
		log.Errorf("Error scraping nodepool: %s", err)
		e.scrapeFailures.Inc()
		e.scrapeFailures.Collect(ch)
	}
	return
}

func main() {
	var (
		listenAddress   = kingpin.Flag("web.listen-address", "The address on which to expose the web interface and generated Prometheus metrics.").Default(":9533").String()
		metricsEndpoint = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		nodepoolHost    = kingpin.Flag("nodepool.listen-host", "The nodepool hostname.").Default("localhost").String()
		nodepoolPort    = kingpin.Flag("nodepool.listen-port", "The nodepool port.").Default("8005").String()
	)

	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("nodepool_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	host := Host{name: *nodepoolHost, port: *nodepoolPort}
	exporter := NewExporter(host)
	prometheus.MustRegister(exporter)

	log.Infoln("Starting Nodepool -> Prometheus Exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())
	log.Infof("Accepting nodepool address: %s:%s", *nodepoolHost, *nodepoolPort)
	log.Infof("Accepting Prometheus Requests on %s", *listenAddress)

	http.Handle(*metricsEndpoint, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Nodepool Exporter</title></head>
			<body>
			<h1>Nodepool Exporter</h1>
			<p><a href="` + *metricsEndpoint + `">Metrics</a></p>
			</body>
			</html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
