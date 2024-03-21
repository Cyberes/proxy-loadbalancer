# -*- coding: utf-8 -*-
"""
    proxy.py
    ~~~~~~~~
    ⚡⚡⚡ Fast, Lightweight, Pluggable, TLS interception capable proxy server focused on
    Network monitoring, controls & Application development, testing, debugging.

    :copyright: (c) 2013-present by Abhinav Singh and contributors.
    :license: BSD, see LICENSE for more details.
"""
import ipaddress
import logging
import threading
import time

import coloredlogs
from proxy import proxy
from redis import Redis

from .background import validate_proxies

coloredlogs.install(level='INFO')


def entry_point() -> None:
    logger = logging.getLogger(__name__)
    redis = Redis(host='localhost', port=6379, decode_responses=True)
    redis.flushall()
    redis.set('balancer_online', 0)
    redis.set('suicide_online', 0)
    threading.Thread(target=validate_proxies, daemon=True).start()
    time.sleep(5)

    while not int(redis.get('balancer_online')):
        logger.warning('Waiting for background thread to populate proxies...')
        time.sleep(5)

    # suicide.SUICIDE_PACT.pact = threading.Thread(target=suicide.check_url_thread, args=("http://127.0.0.1:9000",), daemon=True)
    # suicide.SUICIDE_PACT.pact.start()

    with proxy.Proxy(
            enable_web_server=True,
            port=9000,
            timeout=300,
            hostname=ipaddress.IPv4Address('0.0.0.0'),
            # NOTE: Pass plugins via *args if you define custom flags.
            # Currently plugins passed via **kwargs are not discovered for
            # custom flags by proxy.py
            # See https://github.com/abhinavsingh/proxy.py/issues/871
            plugins=[
                'app.plugins.ProxyLoadBalancer',
            ],
            disable_headers=[
                b'smartproxy-bypass',
                b'smartproxy-disable-bv3hi'
            ]
    ) as _:
        proxy.sleep_loop()
