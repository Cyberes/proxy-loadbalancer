from .config import SMARTPROXY_USER, SMARTPROXY_PASS


def transform_smartproxy(pxy_addr: str):
    return f"http://{SMARTPROXY_USER}:{SMARTPROXY_PASS}@{pxy_addr}"
