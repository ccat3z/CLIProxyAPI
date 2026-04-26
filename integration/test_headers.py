"""Header integration tests: verify 429 responses lack Retry-After header."""

LIMITED_CONFIG = """\
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
            input_tokens: 1k
            models: ["{upstream_model}"]
    models:
      - name: "{upstream_model}"
        alias: "test-haiku"
        input_price_m: 3
        output_price_m: 15
        cache_price_m: 0.3
"""

import json


def _send_chat(srv, model="test-haiku"):
    """Send a chat request via request() to get full headers."""
    body = json.dumps({
        "model": model,
        "messages": [{"role": "user", "content": "Hi"}],
        "max_tokens": 10,
    }).encode()
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {srv.API_KEY}",
    }
    return srv.request("/v1/chat/completions", method="POST", body=body, headers=headers)


def test_429_has_no_retry_after_header(make_server):
    """429 responses must not include a Retry-After header."""
    srv = make_server(LIMITED_CONFIG)
    srv.start()

    for _ in range(20):
        status, headers, _ = _send_chat(srv)
        if status == 429:
            break
    else:
        raise AssertionError("Expected 429 but never got one")

    assert status == 429
    assert headers.get("Retry-After") is None, "429 must not include Retry-After header"
