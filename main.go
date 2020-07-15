package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type Proxy struct {
	proxy *httputil.ReverseProxy
	host  string

	original func(*http.Request)
}

func newProxy(u *url.URL) *Proxy {
	rp := httputil.NewSingleHostReverseProxy(u)
	p := &Proxy{
		proxy:    rp,
		host:     u.Host,
		original: rp.Director,
	}
	rp.Director = p.director
	return p
}

func (p *Proxy) director(r *http.Request) {
	p.original(r)
	if p.host != "" {
		r.Host = p.host
	}
	if r.ContentLength == 0 || r.Body == nil {
		return
	}
	switch r.Header.Get("Content-Encoding") {
	case "gzip":
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("failed to decompress: ioutil.ReadAll(): %s", err)
			return
		}
		dr, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			log.Printf("failed to decompress: gzip.NewReader(): %s", err)
			r.Body = ioutil.NopCloser(bytes.NewReader(b))
			return
		}
		db, err := ioutil.ReadAll(dr)
		if err != nil {
			log.Printf("failed to decompress: ioutil.ReadAll(decompress): %s", err)
			r.Body = ioutil.NopCloser(bytes.NewReader(b))
			return
		}
		r.Body = ioutil.NopCloser(bytes.NewReader(db))
		r.ContentLength = int64(len(db))
		r.Header.Del("Content-Encoding")
	}
}

func (p *Proxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	p.proxy.ServeHTTP(rw, req)
}

var (
	optTarget string
	optAddr   string
)

func run(ctx context.Context) error {
	flag.StringVar(&optTarget, "target", "https://httpbin.org/", `reverse proxy target URL`)
	flag.StringVar(&optAddr, "addr", ":8000", `reverse proxy server address and port`)

	if optTarget == "" {
		return errors.New("no targets. check -target")
	}
	target, err := url.Parse(optTarget)
	if err != nil {
		return err
	}

	p := newProxy(target)
	srv := &http.Server{
		Addr:    optAddr,
		Handler: p,
	}
	log.Printf("reveser proxy is listening %s\n", optAddr)
	return srv.ListenAndServe()
}

func main() {
	err := run(context.Background())
	if err != nil {
		log.Fatal(err)
	}
}
