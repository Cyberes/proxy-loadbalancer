# proxy-loadbalancer

_A round-robin load balancer for HTTP proxies._

This is a simple load balancer using [proxy.py](https://github.com/abhinavsingh/proxy.py) that will route requests to a
cluster of proxy backends in a round-robin fashion. This makes it easy to connect your clients to a large number of
proxy
servers without worrying about implementing anything special clientside.

## Install

1. `pip install -r requirements.txt`
2. Copy `proxy-skeleton/app/config.py.example` to `proxy-skeleton/app/config.py` and fill in your config details.
3. Deploy the `./canihazip` directory and start the server.

## Use

To start the load balancer server, navigate to `./proxy-skeleton` and run `python3 -m app`. The systemd service
`loadbalancer.service` is provided as a service example.

## Special Headers

The load balancer accepts special headers to control its behavior.

- `Smartproxy-Bypass`: don't use any SmartProxy endpoints.
- `Smartproxy-Disable-BV3HI`: don't filter SmartProxy endpoints by the 503 connect error.