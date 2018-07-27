package fetcher

import (
	"net/url"
	"github.com/tomasen/fcgi_client"
	"io/ioutil"
	"github.com/pkg/errors"
	"net/http"
)

type DataFetcher interface {
	GetDataHttp(u *url.URL) ([]byte, error)
	GetDataFastCgi(u *url.URL) ([]byte, error)
}

type dataFetcher struct {

}

func NewDataFetcher() DataFetcher {
	return &dataFetcher{}
}

func (f *dataFetcher) GetDataFastCgi(u *url.URL) ([]byte, error) {
	path := u.Path
	host := u.Host

	if path == "" || u.Scheme == "unix" {
		path = "/status"
	}
	if u.Scheme == "unix" {
		host = u.Path
	}

	env := map[string]string{
		"SCRIPT_FILENAME": path,
		"SCRIPT_NAME":     path,
		"QUERY_STRING":    u.Query().Encode(),
	}

	fcgi, err := fcgiclient.Dial(u.Scheme, host)
	if err != nil {
		return nil, errors.Wrap(err, "fastcgi dial failed")
	}

	defer fcgi.Close()

	resp, err := fcgi.Get(env)
	if err != nil {
		return nil, errors.Wrap(err, "fastcgi get failed")
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 0 {
		return nil, errors.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read fastcgi body")
	}

	return body, nil
}

func (f *dataFetcher) GetDataHttp(u *url.URL) ([]byte, error) {
	req := http.Request{
		Method:     "GET",
		URL:        u,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Host:       u.Host,
	}

	resp, err := http.DefaultClient.Do(&req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read http body")
	}

	return body, nil
}

