"""Shared fixtures for CLIProxyAPI integration tests."""

import json
import os
import signal
import socket
import subprocess
import time
import urllib.request
import urllib.error

import pytest

HOST = "127.0.0.1"

CONFIG_TEMPLATE = """\
host: "{host}"
port: {port}
debug: true
usage-statistics-enabled: true
usage-db: {usage_db}
api-keys:
  - "{api_key}"
openai-compatibility:
  - name: "test-upstream"
    base-url: "{upstream_url}"
    api-key-entries:
      - api-key: "{upstream_key}"
        limits:
          - window: 1h
            input_tokens: 3k
            models: ["{upstream_model}"]
    models:
      - name: "{upstream_model}"
        alias: "test-haiku"
        input_price_m: 3
        output_price_m: 15
        cache_price_m: 0.3
"""

class Server:
    """Manages a CLIProxyAPI subprocess for integration testing."""

    API_KEY = "integration-test-key"

    def __init__(self, workdir, template):
        self.workdir = workdir
        self.config_dir = os.path.join(workdir, "config")
        self.usage_db_dir = os.path.join(workdir, "usage_db")
        os.makedirs(self.config_dir, exist_ok=True)
        os.makedirs(self.usage_db_dir, exist_ok=True)

        self.port = self.find_free_port()
        self.base_url = f"http://{HOST}:{self.port}"
        self.process = None
        self.config_path = os.path.join(self.config_dir, "config.yaml")

        self.write_config(template)

    @staticmethod
    def find_free_port():
        """Find a free TCP port on localhost."""
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            s.bind((HOST, 0))
            return s.getsockname()[1]

    @staticmethod
    def get_upstream_api():
        """Read Anthropic API credentials from environment variables.

        Skips the test if any required env var is missing.
        """
        api_key = os.environ.get("ANTHROPIC_AUTH_TOKEN")
        base_url = os.environ.get("ANTHROPIC_BASE_URL")
        model = os.environ.get("ANTHROPIC_DEFAULT_HAIKU_MODEL")
        model_2 = os.environ.get("ANTHROPIC_DEFAULT_SONNET_MODEL")
        missing = []
        if not api_key:
            missing.append("ANTHROPIC_AUTH_TOKEN")
        if not base_url:
            missing.append("ANTHROPIC_BASE_URL")
        if not model:
            missing.append("ANTHROPIC_DEFAULT_HAIKU_MODEL")
        if not model_2:
            missing.append("ANTHROPIC_DEFAULT_SONNET_MODEL")
        if missing:
            pytest.skip(f"{', '.join(missing)} not set, skipping integration test")
        return {"url": base_url, "key": api_key, "model": model, "model_2": model_2}

    def wait_for_server(self, timeout=30):
        """Poll until the server responds, the process exits, or timeout."""
        url = self.base_url + "/healthz"
        deadline = time.time() + timeout
        while time.time() < deadline:
            if self.process.poll() is not None:
                raise RuntimeError(
                    f"Server process exited with code {self.process.returncode}"
                )
            try:
                req = urllib.request.Request(url)
                with urllib.request.urlopen(req, timeout=2):
                    return True
            except (urllib.error.URLError, ConnectionError, OSError):
                time.sleep(0.5)
        raise RuntimeError(f"Server did not start within {timeout}s")

    def write_config(self, template, **overrides):
        upstream = self.get_upstream_api()
        defaults = dict(
            host=HOST, port=self.port, api_key=self.API_KEY,
            upstream_url=upstream["url"].rstrip("/") + "/v1",
            upstream_key=upstream["key"],
            upstream_model=upstream["model"],
            upstream_model_2=upstream["model_2"],
            usage_db=os.path.join(self.usage_db_dir, "usage.db"),
        )
        defaults.update(overrides)
        with open(self.config_path, "w") as f:
            f.write(template.format(**defaults))

    def start(self, timeout=30):
        log_dir = os.path.join(self.config_dir, "logs")
        os.makedirs(log_dir, exist_ok=True)
        self.stdout_log = open(os.path.join(log_dir, "stdout.log"), "w")
        self.stderr_log = open(os.path.join(log_dir, "stderr.log"), "w")
        self.process = subprocess.Popen(
            ["go", "run", "./cmd/server", "--config", self.config_path, "--no-browser"],
            stdout=self.stdout_log, stderr=self.stderr_log,
            cwd=os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
            start_new_session=True,
        )
        try:
            self.wait_for_server(timeout)
        except RuntimeError:
            self.stop()
            raise
        return self

    def stop(self):
        if self.process and self.process.poll() is None:
            try:
                os.killpg(self.process.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
            try:
                self.process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                try:
                    os.killpg(self.process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                self.process.wait()
            self.process = None
        for f in ("stdout_log", "stderr_log"):
            fh = getattr(self, f, None)
            if fh:
                fh.close()
                setattr(self, f, None)

    def restart(self, timeout=30):
        """Stop and restart using the same config and usage DB."""
        self.stop()
        log_dir = os.path.join(self.config_dir, "logs")
        os.makedirs(log_dir, exist_ok=True)
        self.stdout_log = open(os.path.join(log_dir, "stdout_restart.log"), "w")
        self.stderr_log = open(os.path.join(log_dir, "stderr_restart.log"), "w")
        self.process = subprocess.Popen(
            ["go", "run", "./cmd/server", "--config", self.config_path, "--no-browser"],
            stdout=self.stdout_log, stderr=self.stderr_log,
            cwd=os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
            start_new_session=True,
        )
        self.wait_for_server(timeout)

    def chat_completions(self, model, messages=None, api_key=None, stream=False):
        """Send a chat completion request. Returns (status_code, response_body)."""
        if messages is None:
            messages = [{"role": "user", "content": "Say hello in one word."}]
        body = json.dumps({
            "model": model,
            "messages": messages,
            "max_tokens": 50,
            "stream": stream,
        }).encode()
        headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {api_key or self.API_KEY}",
        }
        status, resp_headers, resp_body = self.request(
            "/v1/chat/completions", method="POST", body=body, headers=headers,
        )
        return status, resp_body

    def request(self, path, method="GET", body=None, headers=None, timeout=60):
        """Send an HTTP request. Returns (status_code, headers_dict, response_body).

        headers_dict is a case-insensitive dict-like object from the response.
        response_body is parsed JSON if possible, otherwise raw bytes.
        """
        req = urllib.request.Request(
            self.base_url + path,
            data=body, headers=headers or {}, method=method,
        )
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                raw = resp.read()
                try:
                    body = json.loads(raw)
                except (json.JSONDecodeError, ValueError):
                    body = raw
                return resp.status, resp.headers, body
        except urllib.error.HTTPError as e:
            raw = e.read()
            try:
                body = json.loads(raw)
            except (json.JSONDecodeError, ValueError):
                body = raw
            return e.code, e.headers, body


@pytest.fixture
def make_server(tmp_path):
    """Factory fixture to create a Server with a custom config template.

    Usage:
        srv = make_server(None)             # default CONFIG_TEMPLATE
        srv = make_server(my_template)      # custom template
        srv.start()
        ...
        srv.stop()

    The server is NOT started automatically — call start() when ready.
    Each server gets a unique free port.
    """
    servers = []

    def _make_server(template=None):
        if template is None:
            template = CONFIG_TEMPLATE

        srv = Server(str(tmp_path / f"server_{len(servers)}"), template)
        servers.append(srv)
        return srv

    yield _make_server

    for srv in servers:
        srv.stop()
