import concurrent
import logging
import random
import time

import requests
from redis import Redis

from .config import PROXY_POOL, SMARTPROXY_POOL, IP_CHECKER, MAX_PROXY_CHECKERS
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
        our_online_backends = {}
        smartproxy_online_backends = {}
        ip_addresses = set()

        def check_proxy(pxy):
            try:
                smartproxy = False
                if pxy in SMARTPROXY_POOL:
                    smartproxy = True
                    r = requests.get(IP_CHECKER, proxies={'http': transform_smartproxy(pxy), 'https': transform_smartproxy(pxy)}, timeout=15)
                    # r_test = requests.get(TEST_LARGE_FILE, proxies={'http': transform_smartproxy(pxy), 'https': transform_smartproxy(pxy)}, timeout=15)
                else:
                    r = requests.get(IP_CHECKER, proxies={'http': pxy, 'https': pxy}, timeout=15)
                    # r_test = requests.get(TEST_LARGE_FILE, proxies={'http': pxy, 'https': pxy}, timeout=15)

                if r.status_code != 200:
                    logger.debug(f'PROXY TEST failed - {pxy} - got code {r.status_code}')
                    return

                # if r_test.status_code != 200:
                #     logger.debug(f'PROXY TEST failed - {pxy} - test download got code {r_test.status_code}')
                #     return

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
        valid_proxies = list(set(our_valid_proxies) | set(smartproxy_valid_proxies))
        if not started:
            random.shuffle(valid_proxies)
            random.shuffle(valid_proxies)
            started = True
        add_backend_cycler('proxy_backends', valid_proxies)

        if logger.level == logging.DEBUG:
            logger.debug(f'Our Backends Online ({len(our_valid_proxies)}): {our_online_backends}')
            logger.debug(f'Smartproxy Backends Online ({len(smartproxy_valid_proxies)}): {smartproxy_valid_proxies}')
        else:
            logger.info(f'Our Backends Online: {len(our_valid_proxies)}, Smartproxy Backends Online: {len(smartproxy_valid_proxies)}, Total: {len(our_valid_proxies) + len(smartproxy_valid_proxies)}')

        redis.set('balancer_online', 1)
        time.sleep(10)

        # if int(redis.get('suicide_online')) == 1 and not suicide.SUICIDE_PACT.pact.is_alive():
        #     logger.critical('Suicide thread not running!')
        #     os.kill(os.getpid(), signal.SIGTERM)
