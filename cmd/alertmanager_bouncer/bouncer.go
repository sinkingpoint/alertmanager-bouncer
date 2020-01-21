package main

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer"
	"gopkg.in/alecthomas/kingpin.v2"
)

type config struct {
	backendURL            *url.URL
	listenURL             *net.TCPAddr
	dialTimeout           time.Duration
	tlsHandshakeTimeout   time.Duration
	responseHeaderTimeout time.Duration
	serverReadTimeout     time.Duration
	serverWriteTimeout    time.Duration
	tlsCertFile           string
	tlsKeyFile            string
	dryRun                bool
}

func main() {
	config := config{}
	app := kingpin.New("alertmanager_bouncer", "A Business Logic Reverse Proxy for Alertmanager")
	app.Flag("backend.addr", "The URL of the backend to upstream to").Required().URLVar(&config.backendURL)
	app.Flag("listen.addr", "The URL for the reverse proxy to listen on").Required().TCPVar(&config.listenURL)
	app.Flag("timeout.dial", "The timeout of the initial connection to the backend").Default("30s").DurationVar(&config.dialTimeout)
	app.Flag("timeout.tlshandshake", "The timeout of the TLS handshake to the backend, after a connection is established").Default("10s").DurationVar(&config.tlsHandshakeTimeout)
	app.Flag("timeout.responseheader", "The timeout of the receive of the initial headers from the backend").Default("10s").DurationVar(&config.responseHeaderTimeout)
	app.Flag("timeout.serverread", "The timeout of the reverse proxy to read requests").Default("5s").DurationVar(&config.serverReadTimeout)
	app.Flag("timeout.serverwrite", "The timeout of the reverse proxy to write the response to the upstream client").Default("10s").DurationVar(&config.serverWriteTimeout)
	app.Flag("tls.certfile", "The file path of the TLS cert file on disk, if you want to serve TLS").ExistingFileVar(&config.tlsCertFile)
	app.Flag("tls.keyfile", "The file path of the TLS key file on disk, if you want to serve TLS").ExistingFileVar(&config.tlsKeyFile)
	app.Flag("dryrun", "If set to true, just log the rejections rather than outright rejecting things").Default("false").BoolVar(&config.dryRun)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	proxy := bouncer.NewBouncingReverseProxy(config.backendURL, []bouncer.Bouncer{
		bouncer.Bouncer{
			Target: bouncer.Target{
				Method:   "POST",
				URIRegex: regexp.MustCompile("/api/v[12]/silences"),
			},
			Deciders: []bouncer.Decider{
				bouncer.AllSilencesHaveAuthorDecider("quirl.co.nz"),
			},
		},
	}, nil)
	server := http.Server{
		ReadTimeout:  config.serverReadTimeout,
		WriteTimeout: config.serverWriteTimeout,
		Handler:      proxy,
		Addr:         config.listenURL.String(),
	}

	var err error
	if config.tlsCertFile != "" && config.tlsKeyFile != "" {
		err = server.ListenAndServeTLS(config.tlsCertFile, config.tlsKeyFile)
	} else {
		if config.tlsCertFile != "" {
			log.Fatalln("TLS Cert file given without TLS Key File. Bailing.")
		} else if config.tlsKeyFile != "" {
			log.Fatalln("TLS Key file given without TLS Config File. Bailing.")
		}
		err = server.ListenAndServe()
	}

	if err != nil {
		log.Fatalf("Got an error while serving HTTP: %s\n", err.Error())
	}
}
