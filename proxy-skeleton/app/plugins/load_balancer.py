import base64
import ipaddress
import logging
from typing import Any, Dict, List, Optional

from proxy.common.constants import (ANY_INTERFACE_HOSTNAMES, COLON, LOCAL_INTERFACE_HOSTNAMES)
from proxy.common.utils import text_
from proxy.core.base import TcpUpstreamConnectionHandler
from proxy.http import Url, httpHeaders, httpMethods
from proxy.http.exception import HttpProtocolException
from proxy.http.parser import HttpParser
from proxy.http.proxy import HttpProxyBasePlugin
from redis import Redis

from ..config import SMARTPROXY_USER, SMARTPROXY_PASS, SMARTPROXY_POOL, BYPASS_SMARTPROXY_DOMAINS
from ..redis_cycle import redis_cycle

logger = logging.getLogger(__name__)

DEFAULT_HTTP_ACCESS_LOG_FORMAT = '{client_ip}:{client_port} - ' + \
                                 '{request_method} {upstream_proxy_host}:{upstream_proxy_port} -> {server_host}:{server_port}{request_path} - ' + \
                                 '{response_code} {response_reason} - {response_bytes} bytes - ' + \
                                 '{connection_time_ms} ms'

DEFAULT_HTTPS_ACCESS_LOG_FORMAT = '{client_ip}:{client_port} - ' + \
                                  '{request_method} {upstream_proxy_host}:{upstream_proxy_port} -> {server_host}:{server_port} - ' + \
                                  '{response_bytes} bytes - {connection_time_ms} ms'


class ProxyLoadBalancer(TcpUpstreamConnectionHandler, HttpProxyBasePlugin):
    """Proxy pool plugin simply acts as a proxy adapter for proxy.py itself.

    Imagine this plugin as setting up proxy settings for proxy.py instance itself.
    All incoming client requests are proxied to configured upstream proxies."""

    def __init__(self, *args: Any, **kwargs: Any) -> None:
        super().__init__(*args, **kwargs)
        self.redis = Redis(host='localhost', port=6379, decode_responses=True)
        self._endpoint: Url = Url()
        self._metadata: List[Any] = [  # Cached attributes to be used during access log override
            None, None, None, None,
        ]

    def handle_upstream_data(self, raw: memoryview) -> None:
        self.client.queue(raw)

    def before_upstream_connection(
            self, request: HttpParser,
    ) -> Optional[HttpParser]:
        """Avoids establishing the default connection to upstream server
        by returning None.

        TODO(abhinavsingh): Ideally connection to upstream proxy endpoints
        must be bootstrapped within it's own re-usable and garbage collected pool,
        to avoid establishing a new upstream proxy connection for each client request.

        See :class:`~proxy.core.connection.pool.UpstreamConnectionPool` which is a work
        in progress for SSL cache handling.
        """
        # We don't want to send private IP requests to remote proxies
        try:
            if ipaddress.ip_address(text_(request.host)).is_private:
                return request
        except ValueError:
            pass

        # Select the proxy to use.
        self._endpoint = self._select_proxy(request.host.decode(), request.has_header(b'smartproxy-bypass'))

        # If chosen proxy is the local instance, bypass upstream proxies
        assert self._endpoint.port and self._endpoint.hostname
        if self._endpoint.port == self.flags.port and \
                self._endpoint.hostname in LOCAL_INTERFACE_HOSTNAMES + ANY_INTERFACE_HOSTNAMES:
            return request

        # Establish connection to chosen upstream proxy
        endpoint_tuple = (text_(self._endpoint.hostname), self._endpoint.port)
        logger.debug('Using endpoint: {0}:{1}'.format(*endpoint_tuple))
        self.initialize_upstream(*endpoint_tuple)
        assert self.upstream

        try:
            self.upstream.connect()
        except TimeoutError:
            raise HttpProtocolException(
                'Timed out connecting to upstream proxy {0}:{1}'.format(
                    *endpoint_tuple,
                ),
            )
        except ConnectionRefusedError:
            # TODO(abhinavsingh): Try another choice, when all (or max configured) choices have
            # exhausted, retry for configured number of times before giving up.

            # Failing upstream proxies, must be removed from the pool temporarily.
            # A periodic health check must put them back in the pool.  This can be achieved
            # using a data structure without having to spawn separate thread/process for health
            # check.
            raise HttpProtocolException(
                'Connection refused by upstream proxy {0}:{1}'.format(
                    *endpoint_tuple,
                ),
            )
        logger.debug(
            'Established connection to upstream proxy {0}:{1}'.format(
                *endpoint_tuple,
            ),
        )
        return None

    def handle_client_request(
            self, request: HttpParser,
    ) -> Optional[HttpParser]:
        """Only invoked once after client original proxy request has been received completely."""
        if not self.upstream:
            return request
        assert self.upstream

        # For log sanity (i.e. to avoid None:None), expose upstream host:port from headers
        host, port = None, None
        # Browser or applications may sometime send
        #
        # "CONNECT / HTTP/1.0\r\n\r\n"
        #
        # for proxy keep alive checks.

        if request.has_header(b'host'):
            url = Url.from_bytes(request.header(b'host'))
            assert url.hostname
            host, port = url.hostname.decode('utf-8'), url.port
            port = port if port else (
                443 if request.is_https_tunnel else 80
            )
        else:
            # TODO: make sure this doesn't break anything
            host = request.host.decode()
            port = request.port

        path = None if not request.path else request.path.decode()

        self._metadata = [
            host, port, path, request.method,
        ]

        # Queue original request optionally with auth headers to upstream proxy
        if self._endpoint.has_credentials:
            assert self._endpoint.username and self._endpoint.password
            request.add_header(
                httpHeaders.PROXY_AUTHORIZATION,
                b'Basic ' +
                base64.b64encode(
                    self._endpoint.username +
                    COLON +
                    self._endpoint.password,
                ),
            )
        self.upstream.queue(memoryview(request.build(for_proxy=True)))
        return request

    def handle_client_data(self, raw: memoryview) -> Optional[memoryview]:
        """Only invoked when before_upstream_connection returns None"""
        # Queue data to the proxy endpoint
        assert self.upstream
        self.upstream.queue(raw)
        return raw

    def handle_upstream_chunk(self, chunk: memoryview) -> Optional[memoryview]:
        """Will never be called since we didn't establish an upstream connection."""
        if not self.upstream:
            return chunk
        raise Exception("This should have never been called")

    def on_upstream_connection_close(self) -> None:
        """Called when client connection has been closed."""
        if self.upstream and not self.upstream.closed:
            logger.debug('Closing upstream proxy connection')
            self.upstream.close()
            self.upstream = None

    def on_access_log(self, context: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        if not self.upstream:
            return context
        addr, port = (self.upstream.addr[0], self.upstream.addr[1]) \
            if self.upstream else (None, None)
        context.update({
            'upstream_proxy_host': addr,
            'upstream_proxy_port': port,
            'server_host': self._metadata[0],
            'server_port': self._metadata[1],
            'request_path': self._metadata[2],
            'response_bytes': self.total_size,
        })
        self.access_log(context)
        return None

    def access_log(self, log_attrs: Dict[str, Any]) -> None:
        access_log_format = DEFAULT_HTTPS_ACCESS_LOG_FORMAT
        request_method = self._metadata[3]
        if request_method and request_method != httpMethods.CONNECT:
            access_log_format = DEFAULT_HTTP_ACCESS_LOG_FORMAT
        for attr, value in log_attrs.items():
            if isinstance(value, bytes):
                log_attrs[attr] = value.decode('utf-8')
        logger.info(access_log_format.format_map(log_attrs))

    def _select_proxy(self, request_host: str = None, smartproxy_bypass: bool = True) -> Url:
        online = int(self.redis.get('balancer_online'))
        if not online:
            logger.error('Server is not online!')
            return Url()

        if request_host in BYPASS_SMARTPROXY_DOMAINS or smartproxy_bypass:
            valid_backends = redis_cycle('our_proxy_backends')
        else:
            valid_backends = redis_cycle('all_proxy_backends')

        if not len(valid_backends):
            logger.error('No valid backends!')
            return Url()

        chosen_backend = valid_backends[0]
        is_smartproxy = chosen_backend in SMARTPROXY_POOL

        if not is_smartproxy:
            return Url(
                scheme='http'.encode(),
                hostname=chosen_backend.split(':')[0].encode(),
                port=int(chosen_backend.split(':')[1]),
            )
        else:
            return Url(
                scheme='http'.encode(),
                username=SMARTPROXY_USER.encode(),
                password=SMARTPROXY_PASS.encode(),
                hostname=chosen_backend.split(':')[0].encode(),
                port=int(chosen_backend.split(':')[1]),
            )

        # start_time = time.time()
        # while not len(backends) and time.time() - start_time < 30:  # wait a max of 30 seconds.
        #     time.sleep(1)  # wait for 1 second before checking again
        #     backends = redis_cycle('all_proxy_backends')
        # if not len(backends):
        #     logger.error('No available proxy after 30 seconds.')
        #     return Url()
