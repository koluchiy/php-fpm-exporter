package exporter

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
	"github.com/koluchiy/php-fpm-exporter/pkg/php-fpm-exporter/collector"
)

type Config struct {
	Addr string
}

// Exporter handles serving the metrics
type Exporter struct {
	collectors []*collector.Collector
	config Config
}

// OptionsFunc is a function passed to new for setting options on a new Exporter.
type OptionsFunc func(*Exporter) error

func New(config Config) (*Exporter, error) {
	if len(config.Addr) == 0 {
		config.Addr = ":9090"
	}
	e := &Exporter{
		config: config,
	}

	return e, nil
}

func (e *Exporter) AddCollector(c *collector.Collector) {
	e.collectors = append(e.collectors, c)
}

var healthzOK = []byte("ok\n")

func (e *Exporter) healthz(w http.ResponseWriter, r *http.Request) {
	w.Write(healthzOK)
}

func (e *Exporter) Run() error {
	for _, c := range e.collectors {
		if err := prometheus.Register(c); err != nil {
			return errors.Wrap(err, "failed to register metrics")
		}
	}

	prometheus.Unregister(prometheus.NewProcessCollector(os.Getpid(), ""))
	prometheus.Unregister(prometheus.NewGoCollector())

	http.HandleFunc("/healthz", e.healthz)
	http.Handle("/metrics", promhttp.Handler())
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

	srv := &http.Server{Addr: e.config.Addr}
	var g errgroup.Group

	g.Go(func() error {
		// TODO: allow TLS
		return srv.ListenAndServe()
	})
	g.Go(func() error {
		<-stopChan
		// XXX: should shutdown time be configurable?
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		return nil
	})

	if err := g.Wait(); err != http.ErrServerClosed {
		return errors.Wrap(err, "failed to run server")
	}

	return nil
}