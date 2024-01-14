import os
import signal

import psutil


def check_zombie(process):
    try:
        return process.status() == psutil.STATUS_ZOMBIE
    except psutil.NoSuchProcess:
        return False


def get_children_pids(pid):
    parent = psutil.Process(pid)
    children = parent.children(recursive=True)
    return [child.pid for child in children]


def zombie_slayer():
    pid = os.getpid()
    children_pids = get_children_pids(pid)
    zombies = []
    for child_pid in children_pids:
        child = psutil.Process(child_pid)
        if check_zombie(child):
            zombies.append(child_pid)

    if zombies:
        import logging
        logger = logging.getLogger(__name__)
        logger.setLevel(logging.INFO)
        logging.critical(f"Zombie processes detected: {zombies}")
        logging.critical("Killing parent process to reap zombies...")
        os.kill(pid, signal.SIGKILL)
