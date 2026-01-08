package es

import (
	"net/http"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	esDroppedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "logshipper_es_dropped_total",
			Help: "Total number of log events dropped due to ES buffer overflow",
		},
	)

	esSentTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "logshipper_es_sent_total",
			Help: "Total number of log events successfully sent to Elasticsearch",
		},
	)

	esBulkErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "logshipper_es_bulk_errors_total",
			Help: "Total number of Elasticsearch bulk request errors",
		},
	)
)

var once atomic.Bool

func InitMetrics() {
	if once.CompareAndSwap(false, true) {
		prometheus.MustRegister(
			esDroppedTotal,
			esSentTotal,
			esBulkErrorsTotal,
		)
	}
}

func StartMetricsServer(addr string) {
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(addr, nil)
}

func IncDropped(n int) {
	esDroppedTotal.Add(float64(n))
}

func IncSent(n int) {
	esSentTotal.Add(float64(n))
}

func IncBulkError() {
	esBulkErrorsTotal.Inc()
}
