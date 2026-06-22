"""Shared fixtures for CLIProxyAPI integration tests."""

import json
import logging
import os
import signal
import subprocess
import sys
import time
import urllib.error
import urllib.request

import pytest

logging.basicConfig(
    level=logging.INFO,
    stream=sys.stderr,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)

from llama import llama_servers  # noqa: F401  (re-exported fixture)
from misc import find_free_port


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

    def __init__(self, workdir, template, upstream):
        self.workdir = workdir
        self.upstream = upstream
        self.config_dir = os.path.join(workdir, "config")
        self.usage_db_dir = os.path.join(workdir, "usage_db")
        os.makedirs(self.config_dir, exist_ok=True)
        os.makedirs(self.usage_db_dir, exist_ok=True)

        self.port = find_free_port()
        self.base_url = f"http://127.0.0.1:{self.port}"
        self.process = None
        self.config_path = os.path.join(self.config_dir, "config.yaml")

        self.write_config(template)

    def wait_for_server(self, timeout=30):
        """Poll until the server responds and models are registered, the process exits, or timeout."""
        healthz_url = self.base_url + "/healthz"
        models_url = self.base_url + "/v1/models"
        deadline = time.time() + timeout
        # Phase 1: wait for healthz
        while time.time() < deadline:
            if self.process.poll() is not None:
                raise RuntimeError(
                    f"Server process exited with code {self.process.returncode}"
                )
            try:
                req = urllib.request.Request(healthz_url)
                with urllib.request.urlopen(req, timeout=2):
                    break
            except (urllib.error.URLError, ConnectionError, OSError):
                time.sleep(0.5)
        else:
            raise RuntimeError(f"Server did not start within {timeout}s")
        # Phase 2: wait for models to be registered.
        # Use the first api-key from the written config so auth works regardless
        # of which keys the test template defines.
        first_api_key = self._first_api_key()
        while time.time() < deadline:
            if self.process.poll() is not None:
                raise RuntimeError(
                    f"Server process exited with code {self.process.returncode}"
                )
            try:
                headers = {}
                if first_api_key:
                    headers["Authorization"] = f"Bearer {first_api_key}"
                req = urllib.request.Request(models_url, headers=headers)
                with urllib.request.urlopen(req, timeout=2) as resp:
                    data = json.loads(resp.read())
                    if data.get("data"):
                        return True
            except urllib.error.HTTPError:
                # Auth or other HTTP error — server is up, keep polling for models.
                pass
            except (urllib.error.URLError, ConnectionError, OSError):
                pass
            time.sleep(0.5)
        raise RuntimeError(f"Models not registered within {timeout}s")

    def _first_api_key(self):
        """Return the first api-key from the written config, or empty string."""
        with open(self.config_path) as f:
            in_keys = False
            for line in f:
                stripped = line.strip()
                if stripped.startswith("api-keys:"):
                    in_keys = True
                    continue
                if in_keys:
                    if stripped.startswith("- "):
                        return stripped[2:].strip().strip('"').strip("'")
                    if stripped and not stripped.startswith("#"):
                        break
        return ""

    def write_config(self, template, **overrides):
        upstream = self.upstream
        defaults = dict(
            host="127.0.0.1", port=self.port, api_key=self.API_KEY,
            upstream_url=upstream["url"].rstrip("/").removesuffix("/v1").rstrip("/") + "/v1",
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

    def chat_completions(self, model, messages=None, api_key=None, stream=False, prompt="hi"):
        """Send a chat completion request. Returns (status_code, headers, response_body)."""
        if messages is None:
            messages = [{"role": "user", "content": prompt}]
        body = json.dumps({
            "model": model,
            "messages": messages,
            "stream": stream,
            "max_tokens": 50
        }).encode()
        headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {api_key or self.API_KEY}",
        }
        status, resp_headers, resp_body = self.request(
            "/v1/chat/completions", method="POST", body=body, headers=headers,
        )
        return status, resp_headers, resp_body

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
def make_server(tmp_path, llama_servers):
    """Factory fixture to create a Server with a custom config template.

    Usage:
        srv = make_server(None)                       # default template, default model
        srv = make_server(None, model="small")        # default template, SmolLM2-135M
        srv = make_server(my_template)                # custom template, default model
        srv = make_server(my_template, model="small") # custom template, SmolLM2-135M
        srv.start()
        ...
        srv.stop()

    The server is NOT started automatically — call start() when ready.
    Each server gets a unique free port. Upstream is a session-scoped
    self-hosted llama-server; ``model`` selects which one (cached per model).
    """
    servers = []

    def _make_server(template=None, model=None):
        if template is None:
            template = CONFIG_TEMPLATE

        upstream = llama_servers(model)
        srv = Server(str(tmp_path / f"server_{len(servers)}"), template, upstream)
        servers.append(srv)
        return srv

    yield _make_server

    for srv in servers:
        srv.stop()
