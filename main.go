package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/saracen/lfscache/server"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		httpAddr     = flag.String("http-addr", ":8080", "HTTP listen address")
		httpsAddr    = flag.String("https-addr", ":8443", "HTTPS listen address (only enabled if key/cert options are provided)")
		tlsKey       = flag.String("tls-key", "", "HTTPS TLS key filepath")
		tlsCert      = flag.String("tls-cert", "", "HTTPS TLS certificate filepath")
		lfsServerURL = flag.String("url", "", "LFS server URL")
		directory    = flag.String("directory", "./objects", "cache directory")
		printVersion = flag.Bool("v", false, "print version")
	)

	flag.Parse()

	if *printVersion {
		fmt.Printf("%v, commit %v, built at %v\n", version, commit, date)
		os.Exit(0)
	}

	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
		logger = level.NewFilter(logger, level.AllowInfo())
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
	}

	addr, err := url.Parse(*lfsServerURL)
	if err == nil && (addr.Scheme != "http" && addr.Scheme != "https") {
		err = errors.New("unsupported LFS server URL")
	}
	if err != nil {
		if logErr := level.Error(logger).Log("err", err); logErr != nil {
			fmt.Fprintf(os.Stderr, "failed to log error: %v\n", logErr)
		}
		os.Exit(1)
	}

	s, err := server.New(logger, addr.String(), *directory)
	if err != nil {
		panic(err)
	}

	switch {
	case *tlsKey != "" && *tlsCert == "":
		*tlsCert = *tlsKey
	case *tlsKey == "" && *tlsCert != "":
		*tlsKey = *tlsCert
	}

	httpsEnabled := *httpsAddr != "" && *tlsKey != ""

	var wg sync.WaitGroup
	if *httpAddr != "" {
		if logErr := level.Info(logger).Log("event", "listening", "proxy-endpoint", addr.String(), "transport", "HTTP", "addr", *httpAddr); logErr != nil {
			fmt.Fprintf(os.Stderr, "failed to log info: %v\n", logErr)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			if httpsEnabled {
				http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					host, _, _ := net.SplitHostPort(r.Host)
					_, port, _ := net.SplitHostPort(*httpsAddr)

					url := r.URL
					url.Scheme = "https"
					url.Host = host + ":" + port

					http.Redirect(w, r, url.String(), http.StatusMovedPermanently)
				})
				panic(http.ListenAndServe(*httpAddr, nil))
			} else {
				panic(http.ListenAndServe(*httpAddr, s.Handle()))
			}
		}()
	}

	if httpsEnabled {
		if logErr := level.Info(logger).Log("event", "listening", "proxy-endpoint", addr.String(), "transport", "HTTPS", "addr", *httpsAddr); logErr != nil {
			fmt.Fprintf(os.Stderr, "failed to log info: %v\n", logErr)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			panic(http.ListenAndServeTLS(*httpsAddr, *tlsCert, *tlsKey, s.Handle()))
		}()
	}

	wg.Wait()
}
