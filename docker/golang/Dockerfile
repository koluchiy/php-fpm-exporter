FROM golang:1.10-alpine as builder

RUN apk --no-cache add bash make openssl
ADD source.tar.gz /go/src/github.com/koluchiy/php-fpm-exporter
#RUN cd /go/src/bitbucket.org/carprice/php-fpm-exporter && ./script/build
RUN cd /go/src/github.com/koluchiy/php-fpm-exporter && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o php-fpm-exporter cmd/main.go
RUN ls -la /go/src/github.com/koluchiy/php-fpm-exporter/php-fpm-exporter

FROM scratch
COPY --from=builder /go/src/github.com/koluchiy/php-fpm-exporter/php-fpm-exporter /php-fpm-exporter

EXPOSE 8888

ENTRYPOINT [ "/php-fpm-exporter" ]
