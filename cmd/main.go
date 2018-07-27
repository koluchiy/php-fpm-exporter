package main

import (
	"github.com/koluchiy/php-fpm-exporter/pkg/php-fpm-exporter/exporter"
	"fmt"
)

func main() {
	factory := exporter.EasyFactory{}

	e, err := factory.GetExporter()

	fmt.Println(err)
	e.Run()
}
