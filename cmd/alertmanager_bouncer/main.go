package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer"
)

var config struct {
	BackendURL         *url.URL     `name:"backend.addr" help:"The URL of the backend to upstream to"`
	ListenURL          *net.TCPAddr `name:"listen.addr" help:"The URL for the reverse proxy to listen on"`
	TlsCertFile        string       `name:"tls.certfile" help:"The file path of the TLS cert file on disk, if you want to serve TLS"`
	TlsKeyFile         string       `name:"tls.keyfile" help:"The file path of the TLS key file on disk, if you want to serve TLS"`
	BouncersConfigFile string       `name:"config" help:"The file containing the list of bouncers to create"`
}

func loadBouncersFromFile() ([]bouncer.Bouncer, error) {
	jsonFile, err := os.Open(config.BouncersConfigFile)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	bytes, err := io.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	return bouncer.ParseBouncers(bytes)
}

func main() {
	kong.Parse(&config)

	var err error
	bouncers, err := loadBouncersFromFile()
	if err != nil {
		log.Panicf("Failed to parse bouncers from %s: %s", config.BouncersConfigFile, err.Error())
	}

	fmt.Printf("Loaded %d bouncers\n", len(bouncers))

	proxy := bouncer.NewBouncingReverseProxy(config.BackendURL, bouncers, nil)
	server := http.Server{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      proxy,
		Addr:         config.ListenURL.String(),
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP)
	go func() {
		for {
			<-sigChan
			log.Printf("Received a SIGHUP. Reloading Bouncers from %s", config.BouncersConfigFile)
			bouncers, err := loadBouncersFromFile()
			if err != nil {
				log.Printf("Failed to parse bouncers from %s: %s. Aboring Reload.", config.BouncersConfigFile, err.Error())
			}

			if err := bouncer.SetBouncers(bouncers, proxy); err != nil {
				log.Printf("Failed to set bouncers on proxy: %s. Aborting Reload.", err.Error())
			}
		}
	}()

	if config.TlsCertFile != "" && config.TlsKeyFile != "" {
		err = server.ListenAndServeTLS(config.TlsCertFile, config.TlsKeyFile)
	} else {
		if config.TlsCertFile != "" {
			log.Fatalln("TLS Cert file given without TLS Key File. Bailing.")
		} else if config.TlsKeyFile != "" {
			log.Fatalln("TLS Key file given without TLS Config File. Bailing.")
		}
		err = server.ListenAndServe()
	}

	if err != nil {
		log.Fatalf("Got an error while serving HTTP: %s\n", err.Error())
	}
}
