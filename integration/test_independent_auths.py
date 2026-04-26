"""Independent auth limits: different API keys have separate rate limits."""

# Config with two API keys that share the same upstream key
TWO_KEY_CONFIG = """\
host: "{host}"
port: {port}
debug: true
usage-statistics-enabled: true
usage-db: {usage_db}
api-keys:
  - "key-low"
  - "key-high"
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


def test_different_keys_share_same_limit_pool(make_server):
    """Two API keys under the same upstream key share the limit pool.

    The limits are per upstream api-key-entry, so both 'key-low' and 'key-high'
    draw from the same 1k input_tokens pool. After exhausting it with one key,
    the other should also be rate limited.
    """
    srv = make_server(TWO_KEY_CONFIG)
    srv.start()

    # Exhaust limit with key-low
    got_429 = False
    for _ in range(20):
        status, _ = srv.chat_completions("test-haiku", api_key="key-low")
        if status == 429:
            got_429 = True
            break
    assert got_429, "key-low should hit rate limit"

    # key-high should also be limited (same upstream pool)
    status, _ = srv.chat_completions("test-haiku", api_key="key-high")
    assert status == 429, "key-high should also be limited (shared upstream pool)"
