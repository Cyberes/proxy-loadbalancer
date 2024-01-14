import logging
import os
import signal
import threading
import time

import requests
from redis import Redis


def check_url_thread(url: str):
    redis = Redis(host='localhost', port=6379, decode_responses=True)
    redis.set('suicide_online', 1)
    logger = logging.getLogger(__name__)
    logger.setLevel(logging.INFO)
    time.sleep(30)  # give the server some time to start up
    logger.info('Created a suicide pact.')
    while True:
        try:
            response = requests.get(url, timeout=10)
            if response.status_code != 404:
                logger.critical(f"Fetch failed with status code: {response.status_code}")
                os.kill(os.getpid(), signal.SIGTERM)
        except requests.exceptions.RequestException as e:
            logger.critical(f"Fetch failed with exception: {e}")
            os.kill(os.getpid(), signal.SIGTERM)
        time.sleep(10)


class SuicidePact:
    def __init__(self):
        self.pact = threading.Thread()


SUICIDE_PACT = SuicidePact()
