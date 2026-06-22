"""Shared helpers for integration tests."""

import socket
import time
import urllib.error
import urllib.request


def find_free_port():
    """Find a free TCP port on localhost."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def wait_for_http(url, timeout=120, headers=None):
    """Poll ``url`` until it returns 2xx, or raise after ``timeout`` seconds."""
    deadline = time.time() + timeout
    last_err = None
    while time.time() < deadline:
        try:
            req = urllib.request.Request(url, headers=headers or {})
            with urllib.request.urlopen(req, timeout=5) as resp:
                if 200 <= resp.status < 300:
                    return True
        except urllib.error.HTTPError as e:
            # Server is up; treat non-5xx as ready.
            if e.code < 500:
                return True
            last_err = e
        except (urllib.error.URLError, ConnectionError, OSError) as e:
            last_err = e
        time.sleep(0.5)
    raise RuntimeError(f"{url} not ready within {timeout}s: {last_err}")
