# IDFC Name Server

`idfcns` is designed as an intermediary DNS server which can be configured
to forward requests to different upstream DNS servers dependent on query type.

## Why?

This is for use in my home network, in which I employ my ISP's 6rd deployment to
use IPv6. The 6rd tunnel dumps my traffic out in Chicago, and this causes some
interesting routes when interacting with location-based load balancing. This
server offers me a way to force all AAAA requests to always go through the
tunnel, and let the rest of them go to the IPv4 name servers.

The intention is to deploy this between my local BIND servers and the Internet,
to allow BIND to continue serving the internal zone and caching responses.

```
                          +------------+     +---------+
            Internet  <---+   IDFCNS   |<----+  BIND9  |
                          +------------+     +---------+
```

## Running

To run `idfcns` simply start it with a provided configuration file:

```
# go run idfcns.go --config=configs/google-public-dns.json
```