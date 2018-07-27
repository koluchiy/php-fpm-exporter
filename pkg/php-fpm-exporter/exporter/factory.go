package exporter

import (
	"net/url"
	"github.com/koluchiy/php-fpm-exporter/pkg/php-fpm-exporter/collector"
	"os"
	"github.com/koluchiy/php-fpm-exporter/pkg/php-fpm-exporter/fetcher"
	"github.com/koluchiy/php-fpm-exporter/pkg/logger"
)

type Factory interface {
	GetExporter() (*Exporter, error)
}

type EasyFactory struct {

}

func (f EasyFactory) GetExporter() (*Exporter, error) {
	log, err := logger.NewLogger()
	if err != nil {
		panic(err)
	}

	fastcgi := os.Getenv("fastcgi")
	addr := os.Getenv("addr")
	namespace := os.Getenv("namespace")

	e, err := New(Config{
		Addr: addr,
	})
	if err != nil {
		panic(err)
	}

	httpEndpoint, err := url.Parse("http://127.0.0.1/metrics")
	if err != nil {
		panic(err)
	}
	fcgiEndpoint, err := url.Parse(fastcgi)

	collectorConfig := collector.Config{
		FcgiEndpoint: fcgiEndpoint,
		HttpEndpoint: httpEndpoint,
		Namespace: namespace,
		ConstLabels: map[string]string{"hello": "world"},
	}
	c := collector.NewCollector(collectorConfig, fetcher.NewDataFetcher(), log)

	e.AddCollector(c)

	return e, nil
}
