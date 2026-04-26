"""Shared fixtures for CLIProxyAPI integration tests."""

import json
import os
import random
import signal
import socket
import string
import subprocess
import time
import urllib.request
import urllib.error

import pytest

HOST = "127.0.0.1"

_DEFAULT_PROMPT_BODY = """
Summary this in 50 words:

Lorem ipsum dolor sit amet, consectetur adipiscing elit. Praesent et fringilla purus. Praesent id sapien vehicula elit pulvinar maximus sed id nunc. Aliquam tristique, ante a dignissim auctor, nisi massa sollicitudin massa, molestie maximus erat justo vel sem. Aenean mattis maximus enim et hendrerit. Donec eleifend, sapien a convallis porta, diam justo feugiat tortor, vulputate ullamcorper felis ante eget sapien. Quisque fringilla ut metus efficitur scelerisque. Nunc laoreet vestibulum nisi sed luctus. Quisque varius, est a auctor laoreet, metus mauris feugiat urna, vitae sagittis quam massa in est. Etiam sodales suscipit tortor, nec tempus turpis facilisis et.
Nulla libero lacus, consectetur et justo vitae, pharetra ornare nisi. Morbi commodo nunc et facilisis mattis. Morbi in molestie quam. Curabitur nisi risus, luctus in consectetur vel, convallis eget lorem. Etiam aliquet vitae libero in condimentum. Donec tempor, urna sed hendrerit viverra, enim felis malesuada eros, sed dictum felis risus vitae justo. Donec tristique enim ut commodo rhoncus. Nullam mollis justo non ipsum scelerisque, sollicitudin euismod neque semper.
Ut porta congue ex, sit amet ultricies elit consectetur non. Aliquam dolor nisi, mollis non elit a, aliquam bibendum quam. Maecenas quis felis quam. Morbi eu tortor vitae lacus sodales aliquam ut a libero. Donec elit urna, ultrices sed molestie ac, sagittis et justo. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Nulla facilisi.
Sed interdum felis at sem pellentesque, volutpat dictum mi condimentum. Quisque non laoreet justo. Nunc dictum velit vel orci aliquam pellentesque. Mauris at tortor in augue mollis eleifend. Aliquam nec nibh non augue tincidunt congue. Quisque vel semper nisi. Vivamus sagittis erat nisi, ut placerat libero tempus sed. Vestibulum blandit a lorem ac posuere.
Fusce vitae varius lorem. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Integer ac dui nec dolor condimentum vulputate nec porttitor ante. Sed posuere interdum arcu, vel iaculis mauris vestibulum sed. Sed et nibh tincidunt, scelerisque lorem nec, pretium turpis. Aliquam vehicula ut dui id efficitur. Maecenas massa dolor, volutpat quis libero ut, euismod aliquet mauris. In dignissim, lorem id dignissim suscipit, dolor ligula cursus ex, at maximus arcu nunc quis mauris. Phasellus sit amet enim lacus. Aliquam sed semper lacus, sit amet hendrerit felis. Praesent gravida placerat mauris vel vehicula.
Ut blandit lorem eu lacus faucibus bibendum. Vestibulum magna elit, placerat sit amet convallis et, convallis ut orci. Mauris eu scelerisque mauris. Fusce sed risus a purus facilisis vestibulum eu ut justo. Maecenas nisl augue, porttitor quis sem sed, scelerisque viverra sapien. Vestibulum fringilla magna non ipsum sollicitudin condimentum. Sed ultricies orci nunc, sit amet accumsan ipsum porta in. Vestibulum fringilla pharetra ipsum quis rhoncus. Morbi elementum tincidunt ligula sed tincidunt. Donec commodo commodo dolor eu ullamcorper.
Morbi molestie quam quis sem malesuada, ut congue ligula sodales. Fusce rhoncus venenatis vestibulum. Suspendisse non nisl elit. Vivamus dictum, nibh eget egestas auctor, mauris erat porttitor leo, consequat gravida sapien ligula eu mi. Aenean eu velit pulvinar, mollis libero at, ultrices metus. Donec hendrerit orci semper, venenatis magna eget, aliquam libero. Sed ac nisi ac mi ultricies ornare maximus aliquam nunc. Praesent condimentum finibus urna id dictum. Nam vel diam eu felis sodales tincidunt. Sed condimentum, ante eget porttitor hendrerit, nisi arcu tincidunt neque, vitae rhoncus ipsum eros in justo. Aliquam erat volutpat. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas.
Praesent ut massa a enim iaculis lacinia sed pulvinar libero. Etiam viverra, purus nec pharetra consequat, enim ipsum vehicula erat, ac elementum metus lacus id risus. Curabitur ligula mauris, vulputate non ullamcorper imperdiet, bibendum vitae neque. Integer tincidunt aliquet luctus. Vestibulum leo nulla, consequat vitae nibh eget, semper eleifend urna. Ut ultricies felis lorem, elementum molestie est egestas nec. Sed nec magna ut magna sollicitudin mattis eu eget risus. Aenean feugiat nisl ut tellus viverra, id pulvinar arcu auctor. Pellentesque est ipsum, sagittis vel massa ut, elementum consectetur augue. Interdum et malesuada fames ac ante ipsum primis in faucibus. Donec commodo, magna ut tempor pulvinar, elit ligula posuere nibh, quis auctor lacus sapien ac mi. Proin lacinia, risus eu porta maximus, velit felis dictum magna, eget viverra nulla ex a lacus. Quisque imperdiet, neque eget viverra cursus, orci diam luctus odio, eu pretium augue mi non ex. Nulla posuere volutpat felis quis porttitor. Aenean consequat, libero in convallis luctus, nisi odio porta ante, ac euismod sem mi non magna.
Etiam finibus quis erat id dapibus. Ut enim justo, luctus nec ex at, tempor dignissim mauris. Suspendisse nec molestie arcu. Aenean ante libero, semper hendrerit metus eu, vestibulum tempus nisi. Mauris rutrum pulvinar porta. Ut venenatis elementum ante. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Donec blandit ex eget orci lobortis, a ultrices dolor faucibus. Donec sit amet metus fermentum, rhoncus sem id, consequat risus. Nam eu sapien tempor justo ornare aliquam.
Aenean massa dolor, placerat in augue et, consequat pharetra nunc. Etiam convallis tempor ipsum. Praesent vehicula rhoncus rhoncus. Morbi ligula magna, mollis sit amet finibus sit amet, auctor ac enim. Nunc scelerisque, odio non congue lobortis, dui risus faucibus eros, sit amet consequat nisl purus vel lorem. Proin eget varius enim, condimentum pretium dui. Interdum et malesuada fames ac ante ipsum primis in faucibus. Sed sed gravida justo, a consequat risus. Aliquam condimentum odio nibh, ullamcorper rutrum odio auctor ac. Cras sem orci, posuere id elit a, scelerisque varius diam. Mauris malesuada diam in odio sodales rutrum. Phasellus tempor orci id elit congue, quis aliquet orci accumsan. Praesent a libero interdum, luctus lectus at, consequat urna. Vivamus arcu lorem, rhoncus gravida gravida dictum, interdum eget nisl. Vivamus aliquam mi in malesuada tempus.
Vestibulum varius dapibus lobortis. In mattis accumsan neque, vitae ornare orci. Vivamus in molestie risus. Mauris ipsum metus, luctus molestie blandit a, rutrum vel erat. Vestibulum eget urna commodo, molestie urna non, rutrum nisi. Maecenas condimentum vitae nisl vel porttitor. In lacinia, nibh et fermentum gravida, sem mi iaculis velit, quis pharetra leo lorem eget lectus. Pellentesque condimentum vel odio ac tincidunt. Nam risus urna, hendrerit mattis sagittis.
"""


def _make_prompt():
    """Return _DEFAULT_PROMPT_BODY with a random prefix to prevent caching."""
    nonce = ''.join(random.choices(string.ascii_lowercase + string.digits, k=16))
    return f"[ref:{nonce}] {_DEFAULT_PROMPT_BODY}"


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
        """Return fixed upstream API credentials.

        Sends a liveness chat completion to verify the upstream is reachable;
        skips the test if it is not.
        """
        url = "https://llm.ccat3z.xyz"
        key = "sk-no-key"
        model = "ut-1"
        model_2 = "ut-2"
        try:
            body = json.dumps({
                "model": model,
                "messages": [{"role": "user", "content": "hi"}],
                "max_tokens": 1,
            }).encode()
            req = urllib.request.Request(
                url.rstrip("/") + "/v1/chat/completions",
                data=body,
                headers={
                    "Content-Type": "application/json",
                    "Authorization": f"Bearer {key}",
                },
            )
            with urllib.request.urlopen(req, timeout=30):
                pass
        except (urllib.error.URLError, ConnectionError, OSError) as e:
            pytest.skip(f"Upstream {url} unreachable: {e}")
        return {"url": url, "key": key, "model": model, "model_2": model_2}

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
        """Send a chat completion request. Returns (status_code, headers, response_body)."""
        if messages is None:
            messages = [{"role": "user", "content": _make_prompt()}]
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
