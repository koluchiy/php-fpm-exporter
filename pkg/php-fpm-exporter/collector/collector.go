package collector

import (
	"strconv"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"strings"
	"net/url"
	"github.com/koluchiy/php-fpm-exporter/pkg/php-fpm-exporter/fetcher"
)

type Config struct {
	Namespace string
	HttpEndpoint *url.URL
	FcgiEndpoint *url.URL
	ConstLabels prometheus.Labels
}

type descs struct {
	up                 *prometheus.Desc
	acceptedConn       *prometheus.Desc
	listenQueue        *prometheus.Desc
	maxListenQueue     *prometheus.Desc
	listenQueueLength  *prometheus.Desc
	phpProcesses       *prometheus.Desc
	maxActiveProcesses *prometheus.Desc
	maxChildrenReached *prometheus.Desc
	slowRequests       *prometheus.Desc
	scrapeFailures     *prometheus.Desc
	requestsSummary    *prometheus.Desc
	lifetimeSummary    *prometheus.Desc
	failureCount       int
}

type Collector struct {
	fetcher fetcher.DataFetcher
	descs *descs
	logger *zap.Logger
	config Config
}

func NewCollector(config Config, fetcher fetcher.DataFetcher, logger *zap.Logger) *Collector {
	collector := &Collector{
		fetcher: fetcher,
		logger: logger,
		config: config,
	}

	collector.initDescs()
	return collector
}

func (c *Collector) newFuncMetric(metricName string, docString string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(c.config.Namespace, "", metricName),
		docString, labels, c.config.ConstLabels,
	)
}

func (c *Collector) initDescs() {
	c.descs = &descs{
		up:                 c.newFuncMetric("up", "able to contact php-fpm", nil),
		acceptedConn:       c.newFuncMetric("accepted_connections_total", "Total number of accepted connections", nil),
		listenQueue:        c.newFuncMetric("listen_queue_connections", "Number of connections that have been initiated but not yet accepted", nil),
		maxListenQueue:     c.newFuncMetric("listen_queue_max_connections", "Max number of connections the listen queue has reached since FPM start", nil),
		listenQueueLength:  c.newFuncMetric("listen_queue_length_connections", "The length of the socket queue, dictating maximum number of pending connections", nil),
		phpProcesses:       c.newFuncMetric("processes_total", "process count", []string{"state"}),
		maxActiveProcesses: c.newFuncMetric("active_max_processes", "Maximum active process count", nil),
		maxChildrenReached: c.newFuncMetric("max_children_reached_total", "Number of times the process limit has been reached", nil),
		slowRequests:       c.newFuncMetric("slow_requests_total", "Number of requests that exceed request_slowlog_timeout", nil),
		scrapeFailures:     c.newFuncMetric("scrape_failures_total", "Number of errors while scraping php_fpm", nil),
		requestsSummary:    c.newFuncMetric("workers_requests_summary", "Summary of requests process by workers", nil),
		lifetimeSummary:    c.newFuncMetric("workers_lifetime_summary", "Workers lifetime summary", nil),

	}
}

func (c *Collector) getMetricName(name string) string {
	return prometheus.BuildFQName(c.config.Namespace, "", name)
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.descs.up
	ch <- c.descs.scrapeFailures
	ch <- c.descs.acceptedConn
	ch <- c.descs.listenQueue
	ch <- c.descs.maxListenQueue
	ch <- c.descs.listenQueueLength
	ch <- c.descs.phpProcesses
	ch <- c.descs.maxActiveProcesses
	ch <- c.descs.maxChildrenReached
	ch <- c.descs.slowRequests
	ch <- c.descs.requestsSummary
	ch <- c.descs.lifetimeSummary
}

func (c *Collector) processPool(body string, ch chan<- prometheus.Metric) {
	statStrings := strings.Split(body, "\n")

	var parsed [][]string

	for _, row := range statStrings {
		parts := strings.Split(row, ":")
		if len(parts) == 2 {
			parsed = append(parsed, []string{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])})
		}
	}

	for _, match := range parsed {
		key := match[0]
		value, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}

		var desc *prometheus.Desc
		var valueType prometheus.ValueType
		var labels []string

		switch key {
		case "accepted conn":
			desc = c.descs.acceptedConn
			valueType = prometheus.CounterValue
		case "listen queue":
			desc = c.descs.listenQueue
			valueType = prometheus.GaugeValue
		case "max listen queue":
			desc = c.descs.maxListenQueue
			valueType = prometheus.CounterValue
		case "listen queue len":
			desc = c.descs.listenQueueLength
			valueType = prometheus.GaugeValue
		case "idle processes":
			desc = c.descs.phpProcesses
			valueType = prometheus.GaugeValue
			labels = append(labels, "idle")
		case "active processes":
			desc = c.descs.phpProcesses
			valueType = prometheus.GaugeValue
			labels = append(labels, "active")
		case "max active processes":
			desc = c.descs.maxActiveProcesses
			valueType = prometheus.CounterValue
		case "max children reached":
			desc = c.descs.maxChildrenReached
			valueType = prometheus.CounterValue
		case "slow requests":
			desc = c.descs.slowRequests
			valueType = prometheus.CounterValue
		default:
			continue
		}

		m, err := prometheus.NewConstMetric(desc, valueType, float64(value), labels...)
		if err != nil {
			c.logger.Error(
				"failed to create metrics",
				zap.String("key", key),
				zap.Error(err),
			)
			continue
		}

		ch <- m
	}
}

func (c *Collector) processWorkers(workers []string, ch chan<- prometheus.Metric) {
	started := 0
	sum := 0
	requestsTotal := 0

	requestsSummary := prometheus.NewSummary(prometheus.SummaryOpts{
		Name:       c.getMetricName("workers_requests_summary"),
		Help:       "summary of requests processed by workers",
		Objectives: map[float64]float64{0.5: 0.05, 0.75: 0.025, 0.9: 0.01, 0.99: 0.001},
		ConstLabels: map[string]string{"pool": "www"},
	})

	lifetimeSummary := prometheus.NewSummary(prometheus.SummaryOpts{
		Name:       c.getMetricName("workers_lifetime_summary"),
		Help:       "summary of workers lifetime",
		Objectives: map[float64]float64{0.5: 0.05, 0.75: 0.025, 0.9: 0.01, 0.99: 0.001},
		ConstLabels: map[string]string{"pool": "www"},
	})

	for _, worker := range workers {
		statStrings := strings.Split(worker, "\n")

		var parsed [][]string

		for _, row := range statStrings {
			parts := strings.Split(row, ":")
			if len(parts) == 2 {
				parsed = append(parsed, []string{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])})
			}
		}

		for _, match := range parsed {
			key := match[0]
			value, err := strconv.Atoi(match[1])
			if err != nil {
				continue
			}

			switch key {
			case "start since":
				started++
				sum += value
				lifetimeSummary.Observe(float64(value))
			case "requests":
				requestsTotal += value
				requestsSummary.Observe(float64(value))
			default:
				continue
			}
		}
	}

	ch <- lifetimeSummary
	ch <- requestsSummary
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	up := 1.0
	var (
		body []byte
		err  error
	)

	if c.config.FcgiEndpoint != nil {
		body, err = c.fetcher.GetDataFastCgi(c.config.FcgiEndpoint)
	} else {
		body, err = c.fetcher.GetDataHttp(c.config.HttpEndpoint)
	}

	if err != nil {
		up = 0.0
		c.logger.Error("failed to get php-fpm status", zap.Error(err))
		c.descs.failureCount++
	}
	ch <- prometheus.MustNewConstMetric(
		c.descs.up,
		prometheus.GaugeValue,
		up,
	)

	ch <- prometheus.MustNewConstMetric(
		c.descs.scrapeFailures,
		prometheus.CounterValue,
		float64(c.descs.failureCount),
	)

	if up == 0.0 {
		return
	}

	blocks := strings.Split(string(body), "******")

	c.processPool(blocks[0], ch)
	c.processWorkers(blocks[1:], ch)
}