"""Rate limiting integration tests: token and price limits enforced via 429."""

_MULTI_MODEL_TEMPLATE = """\
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
            models: ["{upstream_model}", "{upstream_model_2}"]
    models:
      - name: "{upstream_model}"
        alias: "test-haiku"
        input_price_m: 3
        output_price_m: 15
        cache_price_m: 0.3
      - name: "{upstream_model_2}"
        alias: "test-sonnet"
        input_price_m: 3
        output_price_m: 15
        cache_price_m: 0.3
"""

_WILDCARD_TEMPLATE = """\
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
            models: []
    models:
      - name: "{upstream_model}"
        alias: "test-haiku"
        input_price_m: 3
        output_price_m: 15
        cache_price_m: 0.3
"""


def _drain_until_429(srv, model, api_key=None):
    """Send chat requests until 429, tracking tokens from responses."""
    total_input = 0
    for _ in range(10):
        status, _, body = srv.chat_completions(model, api_key=api_key)
        if status == 429:
            return status, body, total_input
        if status != 200:
            return status, body, total_input
        total_input += body.get("usage", {}).get("prompt_tokens", 0)
    return status, body, total_input


def test_input_tokens_limit_returns_429(make_server):
    """When input token usage reaches the configured 3k limit, next request gets 429."""
    srv = make_server(None)
    srv.start()
    status, body, total_input = _drain_until_429(srv, "test-haiku")
    assert status == 429, (
        f"Expected 429 after exceeding input_tokens limit (accumulated {total_input})"
    )
    assert "rate limit" in body.get("error", {}).get("message", "").lower()


def test_single_request_within_limits(make_server):
    """A single request should succeed within the 3k input_tokens limit."""
    srv = make_server(None)
    srv.start()
    status, _, _ = srv.chat_completions("test-haiku")
    assert status == 200


def test_limit_error_includes_details(make_server):
    """429 response body includes limit type, current, and limit values."""
    srv = make_server(None)
    srv.start()
    status, body, _ = _drain_until_429(srv, "test-haiku")
    assert status == 429, "Expected 429 to check error details"
    error = body.get("error", {})
    msg = error.get("message", "")
    assert "input_tokens" in msg or "price" in msg, f"Unexpected error msg: {msg}"


def test_shared_window_models_trigger_429(make_server):
    """Models in the same limit window share usage: combined tokens trigger 429."""
    srv = make_server(_MULTI_MODEL_TEMPLATE)
    srv.start()
    total_input = 0
    got_429 = False
    for i in range(10):
        model = "test-haiku" if i % 2 == 0 else "test-sonnet"
        status, _, body = srv.chat_completions(model)
        if status == 429:
            got_429 = True
            assert "rate limit" in body.get("error", {}).get("message", "").lower()
            break
        if status != 200:
            break
        total_input += body.get("usage", {}).get("prompt_tokens", 0)
    assert got_429, (
        f"Expected 429 when combined usage exceeds limit (accumulated {total_input})"
    )


def test_wildcard_limit_trigger_429(make_server):
    """Wildcard (empty models) limit applies to all models for the authID."""
    srv = make_server(_WILDCARD_TEMPLATE)
    srv.start()
    status, body, total_input = _drain_until_429(srv, "test-haiku")
    assert status == 429, (
        f"Expected 429 when wildcard limit exceeded (accumulated {total_input})"
    )
    assert "rate limit" in body.get("error", {}).get("message", "").lower()
