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


def test_429_has_no_retry_after_header(make_server):
    """429 responses must not include a Retry-After header."""
    srv = make_server(LIMITED_CONFIG)
    srv.start()

    total_input = 0
    for _ in range(10):
        status, headers, body = srv.chat_completions("test-haiku")
        if status == 429:
            break
        if status != 200:
            break
        total_input += body.get("usage", {}).get("prompt_tokens", 0)
    else:
        raise AssertionError(
            f"Expected 429 but never got one (accumulated {total_input} input tokens)"
        )

    assert status == 429
    assert headers.get("Retry-After") is None, "429 must not include Retry-After header"
