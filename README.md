# proxy-loadbalancer

_A round-robin load balancer for HTTP proxies._

This is a simple proxy load balancer that will route requests to a cluster of proxy backends in a round-robin fashion.
This makes it easy to connect your clients to a large number of proxy servers without worrying about implementing
anything special clientside.

- Downstream HTTPS proxy servers are not supported.
- This proxy server will transparently forward HTTPS requests without terminating them, meaning a self-signed certificate is not required.

## Install

1.  Download the latest release from [/releases](https://git.evulid.cc/cyberes/proxy-loadbalancer/releases) or run `./build.sh` to build the program locally.
2.  `cp config.example.yml config.yml`
3.  Edit the config.
4.  Start the loadbalancer with `./proxy-loadbalancer --config [path to your config.yml]`

## Use

You can run your own "public IP delivery server" `canihazip` <https://git.evulid.cc/cyberes/canihazip> or use the default `api.ipify.org`

An example systemd service `loadbalancer.service` is provided.

The server displays stats and info at `/json`

```
=== Proxy Load Balancer ===
Usage of ./proxy-loadbalancer:
  --config [string]
               Path to the config file
  -d, --debug  
               Enable debug mode
  --v          Print version and exit
  -h, --help   Print this help message
```

## Special Headers

The load balancer accepts special headers to control its behavior.

-   `Thirdparty-Bypass`: don't use any third-party endpoints for this request.
-   `Thirdparty-Include-Broken`: use all online endpoints for this request, including third-party ones that failed the special test.
