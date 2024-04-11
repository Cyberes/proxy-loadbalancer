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

DEBUG_MODE = False

headers = {
    'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36',
    'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8',
    'Accept-Language': 'en-US,en;q=0.5',
    'Connection': 'keep-alive',
    'Upgrade-Insecure-Requests': '1',
    'Sec-Fetch-Dest': 'document',
    'Sec-Fetch-Mode': 'navigate',
    'Sec-Fetch-Site': 'cross-site',
    'Pragma': 'no-cache',
    'Cache-Control': 'no-cache',
}


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
        smartproxy_broken_proxies = {}
        ip_addresses = set()

        def check_proxy(pxy):
            try:
                smartproxy = False
                if pxy in SMARTPROXY_POOL:
                    smartproxy = True
                    r = requests.get(IP_CHECKER, proxies={'http': transform_smartproxy(pxy), 'https': transform_smartproxy(pxy)}, timeout=15, headers=headers)
                else:
                    r = requests.get(IP_CHECKER, proxies={'http': pxy, 'https': pxy}, timeout=15, headers=headers)

                if r.status_code != 200:
                    logger.info(f'PROXY TEST failed - {pxy} - got code {r.status_code}')
                    return
            except Exception as e:
                logger.info(f'PROXY TEST failed - {pxy} - {e}')  # ': {e.__class__.__name__}')
                return

            ip = r.text
            if ip not in ip_addresses:
                proxy_dict = our_online_backends if not smartproxy else smartproxy_online_backends
                ip_addresses.add(ip)
                proxy_dict[pxy] = ip
            else:
                s = ' Smartproxy ' if smartproxy else ' '
                logger.warning(f'Duplicate{s}IP: {ip}')
                return

            # TODO: remove when fixed
            try:
                if smartproxy:
                    for d in SMARTPROXY_BV3HI_FIX:
                        r2 = requests.get(d, proxies={'http': transform_smartproxy(pxy), 'https': transform_smartproxy(pxy)}, timeout=15, headers=headers)
                        if r2.status_code != 200:
                            smartproxy_broken_proxies[pxy] = r.text
                            logger.info(f'PROXY BV3HI TEST failed - {pxy} - got code {r2.status_code}')
            except Exception as e:
                smartproxy_broken_proxies[pxy] = r.text
                logger.info(f'PROXY BV3HI TEST failed - {pxy} - {e}')

        with concurrent.futures.ThreadPoolExecutor(max_workers=MAX_PROXY_CHECKERS) as executor:
            executor.map(check_proxy, set(PROXY_POOL) | set(SMARTPROXY_POOL))

        our_valid_proxies = list(our_online_backends.keys())

        # Remove the broken SmartProxy proxies from the working ones.
        sp_all = list(smartproxy_online_backends.keys())
        smartproxy_broken_proxies = list(smartproxy_broken_proxies.keys())
        smartproxy_valid_proxies = list(set(sp_all) - set(smartproxy_broken_proxies))

        all_valid_proxies = list(set(our_valid_proxies) | set(smartproxy_valid_proxies))
        all_valid_proxies_with_broken_smartproxy = list(set(all_valid_proxies) | set(sp_all))

        if not started:
            random.shuffle(all_valid_proxies)
            random.shuffle(our_valid_proxies)
            started = True

        add_backend_cycler('all_valid_proxies', all_valid_proxies)
        add_backend_cycler('our_valid_proxies', our_valid_proxies)
        add_backend_cycler('all_valid_proxies_with_broken_smartproxy', all_valid_proxies_with_broken_smartproxy)

        if DEBUG_MODE:
            logger.info(f'Our Backends Online ({len(our_valid_proxies)}): {all_valid_proxies}')
            logger.info(f'Smartproxy Backends Online ({len(smartproxy_valid_proxies)}): {smartproxy_valid_proxies}')
            logger.info(f'Smartproxy Broken Backends ({len(smartproxy_broken_proxies)}): {smartproxy_broken_proxies}')
        else:
            logger.info(f'Our Backends Online: {len(our_valid_proxies)}, Smartproxy Backends Online: {len(smartproxy_valid_proxies)}, Smartproxy Broken Backends: {len(smartproxy_broken_proxies)}, Total Online: {len(our_valid_proxies) + len(smartproxy_valid_proxies)}')

        redis.set('balancer_online', 1)
        time.sleep(60)
