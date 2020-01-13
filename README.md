# Alertmanager Bouncer

Sometimes you want to apply business logic to Alertmanager, enforcing authors, logic around silence times etc.
According to upstream, this logic doesn't belong in Alertmanager itself - fair enough, but that leaves us wondering
where to put it.

AlertManager bouncer is a reverse proxy which has the ability to intercept POSTs to the Alertmanager API, and accept, or
reject them based on a set of policies.  