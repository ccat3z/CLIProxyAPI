"""Self-hosted llama-server fixture for integration tests.

Downloads the llama-server binary, model, and mmproj into
``./integration/data/llama-server/`` (cached and sha1-verified), starts
``llama-server`` on a random free port, and yields upstream connection info.

llama-server accepts any API key and any model name, so tests can use
arbitrary values.
"""

import hashlib
import logging
import os
import signal
import subprocess
import sys
import tarfile
import time
import urllib.request

import pytest

from misc import find_free_port, wait_for_http

log = logging.getLogger(__name__)

LLAMA_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "data", "llama-server")
LIBEXEC_DIR = os.path.join(LLAMA_DIR, "libexec")

LLAMA_BIN_URL = "https://github.com/ggml-org/llama.cpp/releases/download/b9754/llama-b9754-bin-ubuntu-x64.tar.gz"
LLAMA_BIN_SHA1 = "343e16d3fe755887eda869c2767e998772d08002"

# Model options. Select via the ``LLAMA_MODEL`` env var (default: ``tiny``).
# Each entry has:
#   model_url, model_sha1, model_filename: the GGUF weights
#   mmproj_url, mmproj_sha1, mmproj_filename: optional multimodal projector
#   extra_args: llama-server flags appended after --port/-m/-mm
MODEL_OPTIONS = {
    "tiny": {
        "model_url": "https://huggingface.co/ggml-org/models/resolve/main/tinyllamas/stories260K.gguf",
        "model_sha1": "307f7acf113c33f369df626421fb4d3bcac345c1",
        "model_filename": "stories260K.gguf",
        "mmproj_url": None,
        "mmproj_sha1": None,
        "mmproj_filename": None,
        "extra_args": [],
    },
    "small": {
        "model_url": "https://huggingface.co/bartowski/SmolLM2-135M-Instruct-GGUF/resolve/main/SmolLM2-135M-Instruct-f16.gguf",
        "model_sha1": "80f1ddfdbfc21773b548de403046a2971e30bd3c",
        "model_filename": "smollm2-135m-f16.gguf",
        "mmproj_url": None,
        "mmproj_sha1": None,
        "mmproj_filename": None,
        "extra_args": [
            "--presence-penalty", "0.5",
            "--repeat-penalty", "1.05",
        ],
    },
    "mllm": {
        "model_url": "https://huggingface.co/unsloth/Qwen3.5-0.8B-GGUF/resolve/main/Qwen3.5-0.8B-Q8_0.gguf",
        "model_sha1": "e5a44d6680b254974574a6a0ce5212813abd102c",
        "model_filename": "qwen3.5-0.8b-q8_0.gguf",
        "mmproj_url": "https://huggingface.co/unsloth/Qwen3.5-0.8B-GGUF/resolve/main/mmproj-BF16.gguf",
        "mmproj_sha1": "0341e1c3889dca4b30a482f213f1f427a67c4824",
        "mmproj_filename": "mmproj-bf16.gguf",
        "extra_args": [
            "--temp", "0.6",
            "--top-p", "0.95",
            "--top-k", "20",
            "--min-p", "0.00",
            "--presence-penalty", "1.5",
            "--repeat-penalty", "1.0",
            "--ctx-size", "4096",
        ],
    },
}

DEFAULT_MODEL = "tiny"


def _sha1(path):
    """Return the sha1 hex digest of the file at ``path``."""
    h = hashlib.sha1()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def _download(url, dest, expected_sha1):
    """Download ``url`` to ``dest`` if missing or sha1-mismatched.

    Skips the download when ``dest`` exists and its sha1 matches
    ``expected_sha1``. Logs progress periodically.
    """
    if os.path.exists(dest) and _sha1(dest) == expected_sha1:
        return
    os.makedirs(os.path.dirname(dest), exist_ok=True)
    tmp = dest + ".tmp"
    log.info("downloading %s -> %s", url, dest)
    with urllib.request.urlopen(url, timeout=600) as r:
        total = int(r.headers.get("Content-Length") or 0)
        downloaded = 0
        last_log = 0.0
        with open(tmp, "wb") as f:
            for chunk in iter(lambda: r.read(1 << 20), b""):
                f.write(chunk)
                downloaded += len(chunk)
                now = time.time()
                if now - last_log >= 1.0:
                    if total > 0:
                        log.info(
                            "progress %.1fMB/%.1fMB (%.1f%%)",
                            downloaded / (1 << 20),
                            total / (1 << 20),
                            downloaded * 100 / total,
                        )
                    else:
                        log.info("progress %.1fMB", downloaded / (1 << 20))
                    last_log = now
    actual = _sha1(tmp)
    if actual != expected_sha1:
        os.remove(tmp)
        raise RuntimeError(
            f"sha1 mismatch for {url}: expected {expected_sha1}, got {actual}"
        )
    os.replace(tmp, dest)


def ensure_llama_server_binary():
    """Download and extract the llama-server binary if not present. Returns (binary_path, lib_dir)."""
    bin_path = os.path.join(LIBEXEC_DIR, "llama-server")
    if os.path.exists(bin_path) and os.access(bin_path, os.X_OK):
        return bin_path, LIBEXEC_DIR
    os.makedirs(LIBEXEC_DIR, exist_ok=True)
    tar_path = os.path.join(LLAMA_DIR, "llama-server.tar.gz")
    _download(LLAMA_BIN_URL, tar_path, LLAMA_BIN_SHA1)
    with tarfile.open(tar_path) as t:
        t.extractall(LIBEXEC_DIR)
    # The tarball lays out binaries under a versioned subdir (e.g. llama-b9754/).
    # Promote its contents to LIBEXEC_DIR so the binary and shared libs live in
    # a single directory usable as LD_LIBRARY_PATH.
    subdirs = [d for d in os.listdir(LIBEXEC_DIR)
               if os.path.isdir(os.path.join(LIBEXEC_DIR, d)) and d.startswith("llama-b")]
    if subdirs:
        extracted_root = os.path.join(LIBEXEC_DIR, subdirs[0])
        for name in os.listdir(extracted_root):
            src = os.path.join(extracted_root, name)
            dst = os.path.join(LIBEXEC_DIR, name)
            if os.path.exists(dst):
                continue
            os.rename(src, dst)
        os.rmdir(extracted_root)
    if not os.path.exists(bin_path):
        raise RuntimeError(
            f"llama-server binary not found after extracting {tar_path} into {LIBEXEC_DIR}"
        )
    os.chmod(bin_path, 0o755)
    return bin_path, LIBEXEC_DIR


def _stop_process(proc):
    """Best-effort terminate-then-kill of a subprocess started with start_new_session."""
    if proc is None or proc.poll() is not None:
        return
    try:
        os.killpg(proc.pid, signal.SIGTERM)
    except ProcessLookupError:
        return
    try:
        proc.wait(timeout=10)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(proc.pid, signal.SIGKILL)
        except ProcessLookupError:
            return
        proc.wait()


def start_llama_server(model_name):
    """Start a llama-server with the given model option. Returns (upstream_dict, cleanup).

    ``cleanup`` is a no-arg callable that stops the subprocess and closes logs.
    """
    if model_name not in MODEL_OPTIONS:
        raise RuntimeError(
            f"unknown model {model_name!r}; choose one of {list(MODEL_OPTIONS)}"
        )
    opt = MODEL_OPTIONS[model_name]
    log.info("using model option %r", model_name)

    bin_path, lib_dir = ensure_llama_server_binary()
    model_path = os.path.join(LLAMA_DIR, opt["model_filename"])
    _download(opt["model_url"], model_path, opt["model_sha1"])
    if opt["mmproj_url"]:
        mmproj_path = os.path.join(LLAMA_DIR, opt["mmproj_filename"])
        _download(opt["mmproj_url"], mmproj_path, opt["mmproj_sha1"])
    else:
        mmproj_path = None

    port = find_free_port()
    base_url = f"http://127.0.0.1:{port}"
    log_dir = os.path.join(LLAMA_DIR, "logs")
    os.makedirs(log_dir, exist_ok=True)
    stdout_log = open(os.path.join(log_dir, f"llama_{port}.stdout.log"), "a")
    stderr_log = open(os.path.join(log_dir, f"llama_{port}.stderr.log"), "a")

    cmd = [bin_path, "--port", str(port), "-m", model_path]
    if mmproj_path:
        cmd += ["-mm", mmproj_path]
    cmd += opt["extra_args"]
    env = os.environ.copy()
    env["LD_LIBRARY_PATH"] = lib_dir + (":" + env["LD_LIBRARY_PATH"] if env.get("LD_LIBRARY_PATH") else "")
    proc = subprocess.Popen(
        cmd,
        stdout=stdout_log,
        stderr=stderr_log,
        start_new_session=True,
        env=env,
    )

    def cleanup():
        _stop_process(proc)
        stdout_log.close()
        stderr_log.close()

    try:
        wait_for_http(base_url + "/health", timeout=180)
    except Exception:
        cleanup()
        raise

    return {
        "url": base_url + "/v1",
        "key": "test-llama-key",
        "model": "test-model",
        "model_2": "test-model-2",
    }, cleanup


@pytest.fixture(scope="session")
def llama_servers():
    """Session-scoped cache of running llama-server instances keyed by model name.

    Yields a function ``get(model_name=DEFAULT_MODEL)`` that returns the upstream
    dict for ``model_name``, starting the server on first use and reusing it
    afterward. All started servers are stopped at session teardown.
    """
    cache = {}

    def get(model_name=None):
        if model_name is None:
            model_name = os.environ.get("LLAMA_MODEL", DEFAULT_MODEL)
        if model_name not in cache:
            cache[model_name] = start_llama_server(model_name)
        return cache[model_name][0]

    try:
        yield get
    finally:
        for _, cleanup in cache.values():
            cleanup()
