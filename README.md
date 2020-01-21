# Alertmanager Bouncer

Sometimes you want to apply business logic to Alertmanager, enforcing authors, logic around silence times etc.
According to upstream, this logic doesn't belong in Alertmanager itself - fair enough, but that leaves us wondering
where to put it.

AlertManager bouncer is a reverse proxy which has the ability to intercept POSTs to the Alertmanager API, and accept, or
reject them based on a set of policies.

## Command Line Help

```
usage: alertmanager_bouncer --backend.addr=BACKEND.ADDR --listen.addr=LISTEN.ADDR --config.bouncersfile=CONFIG.BOUNCERSFILE [<flags>]

A Business Logic Reverse Proxy for Alertmanager

Flags:
  --help                        Show context-sensitive help (also try --help-long and --help-man).
  --backend.addr=BACKEND.ADDR   The URL of the backend to upstream to
  --listen.addr=LISTEN.ADDR     The URL for the reverse proxy to listen on
  --config.bouncersfile=CONFIG.BOUNCERSFILE  
                                The file containing the list of bouncers to create
  --timeout.dial=30s            The timeout of the initial connection to the backend
  --timeout.tlshandshake=10s    The timeout of the TLS handshake to the backend, after a connection is established
  --timeout.responseheader=10s  The timeout of the receive of the initial headers from the backend
  --timeout.serverread=5s       The timeout of the reverse proxy to read requests
  --timeout.serverwrite=10s     The timeout of the reverse proxy to write the response to the upstream client
  --tls.certfile=TLS.CERTFILE   The file path of the TLS cert file on disk, if you want to serve TLS
  --tls.keyfile=TLS.KEYFILE     The file path of the TLS key file on disk, if you want to serve TLS
```

## Example

To define the bouncers for your proxy, you need to define them in YAML in the file passed to config.bouncersfile.
The format of this file is as follows:

```yaml
# The bouncers for our proxy
bouncers:
  # Bouncer which enforces that all silences have an author that ends with @cloudflare.com
  - method: POST
    uriRegex: /api/v[12]/silences # Handles both the v1 and v2 API
    deciders:
      - name: AllSilencesHaveAuthor
        config:
          domain: "@cloudflare.com"
    dryrun: false # DryRun = True forces this decider to just log failures, rather than blocking
  # Bouncer which mirrors both silences and alerts to another alertmanager (Maybe for testing)
  - method: POST
    uriRegex: /api/v[12]/(:?silences|alerts)
    deciders:
      - name: Mirror
        config:
          destination: "http://alertmanager-2:9091"
```

## License

Apache License 2.0, see [LICENSE](https://github.com/sinkingpoint/alertmanager_bouncer/blob/master/LICENSE).
