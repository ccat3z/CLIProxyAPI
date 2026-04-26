"""Cost-related integration tests: price limits and cost freezing."""

# Config with a price limit
PRICE_LIMIT_CONFIG = """\
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
            price: 0.01
            models: ["{upstream_model}"]
    models:
      - name: "{upstream_model}"
        alias: "test-haiku"
        input_price_m: 3
        output_price_m: 15
        cache_price_m: 0.3
"""


def test_price_limit_returns_429(make_server):
    """When accumulated cost exceeds the price limit, 429 is returned."""
    srv = make_server(PRICE_LIMIT_CONFIG)
    srv.start()

    got_429 = False
    for _ in range(10):
        status, body = srv.chat_completions("test-haiku")
        if status == 429:
            got_429 = True
            error = body.get("error", {})
            msg = error.get("message", "")
            assert "price" in msg, f"Expected price in error, got: {msg}"
            break
    assert got_429, "Should hit price limit with $0.01 cap"
