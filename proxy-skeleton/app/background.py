import concurrent
import logging
import os
import random
import signal
import time

import requests
from redis import Redis

from .config import PROXY_POOL, SMARTPROXY_POOL, IP_CHECKER, MAX_PROXY_CHECKERS, SMARTPROXY_BV3HI_FIX
from .pid import zombie_slayer
from .redis_cycle import add_backend_cycler
from .smartproxy import transform_smartproxy


def validate_proxies():
    """
    Validate proxies by sending a request to https://api.ipify.org and checking the resulting IP address.
    """
    logger = logging.getLogger(__name__)
    logger.setLevel(logging.INFO)
    redis = Redis(host='localhost', port=6379, decode_responses=True)
    logger.info('Doing inital backend check, please wait...')
    started = False
    while True:
        # Health checks. If one of these fails, the process is killed to be restarted by systemd.
        if int(redis.get('balancer_online')):
            zombie_slayer()
            try:
                response = requests.get('http://localhost:9000', headers={'User-Agent': 'HEALTHCHECK'}, timeout=10)
                if response.status_code != 404:
                    logger.critical(f"Frontend HTTP check failed with status code: {response.status_code}")
                    os.kill(os.getpid(), signal.SIGKILL)
            except requests.exceptions.RequestException as e:
                logger.critical(f"Frontend HTTP check failed with exception: {e}")
                os.kill(os.getpid(), signal.SIGKILL)

        our_online_backends = {}
        smartproxy_online_backends = {}
        ip_addresses = set()

        def check_proxy(pxy):
            try:
                smartproxy = False
                if pxy in SMARTPROXY_POOL:
                    smartproxy = True
                    r = requests.get(IP_CHECKER, proxies={'http': transform_smartproxy(pxy), 'https': transform_smartproxy(pxy)}, timeout=15)

                    # TODO: remove when fixed
                    r2 = requests.get(SMARTPROXY_BV3HI_FIX, proxies={'http': transform_smartproxy(pxy), 'https': transform_smartproxy(pxy)}, timeout=15)
                    if r2.status_code != 200:
                        logger.debug(f'PROXY BV3HI TEST failed - {pxy} - got code {r2.status_code}')
                        return
                else:
                    r = requests.get(IP_CHECKER, proxies={'http': pxy, 'https': pxy}, timeout=15)

                if r.status_code != 200:
                    logger.debug(f'PROXY TEST failed - {pxy} - got code {r.status_code}')
                    return

                ip = r.text
                if ip not in ip_addresses:
                    proxy_dict = our_online_backends if not smartproxy else smartproxy_online_backends
                    ip_addresses.add(ip)
                    proxy_dict[pxy] = ip
                else:
                    s = ' Smartproxy ' if smartproxy else ' '
                    logger.debug(f'Duplicate{s}IP: {ip}')
            except Exception as e:
                logger.debug(f'PROXY TEST failed - {pxy} - {e}')  # ': {e.__class__.__name__}')
                # traceback.print_exc()

        with concurrent.futures.ThreadPoolExecutor(max_workers=MAX_PROXY_CHECKERS) as executor:
            executor.map(check_proxy, set(PROXY_POOL) | set(SMARTPROXY_POOL))

        our_valid_proxies = list(our_online_backends.keys())
        smartproxy_valid_proxies = list(smartproxy_online_backends.keys())
        all_valid_proxies = list(set(our_valid_proxies) | set(smartproxy_valid_proxies))
        if not started:
            random.shuffle(all_valid_proxies)
            random.shuffle(our_valid_proxies)
            started = True
        add_backend_cycler('all_proxy_backends', all_valid_proxies)
        add_backend_cycler('our_proxy_backends', our_valid_proxies)

        if logger.level == logging.DEBUG:
            logger.debug(f'Our Backends Online ({len(our_valid_proxies)}): {our_online_backends}')
            logger.debug(f'Smartproxy Backends Online ({len(smartproxy_valid_proxies)}): {smartproxy_valid_proxies}')
        else:
            logger.info(f'Our Backends Online: {len(our_valid_proxies)}, Smartproxy Backends Online: {len(smartproxy_valid_proxies)}, Total: {len(our_valid_proxies) + len(smartproxy_valid_proxies)}')

        redis.set('balancer_online', 1)
        time.sleep(10)
