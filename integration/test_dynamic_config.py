"""Dynamic config integration tests: hot-reload of config.yaml at runtime."""

import json
import time

CONFIG_WITH_LIMITS = """\
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
            models: [test-haiku]
    models:
      - name: "{upstream_model}"
        alias: "test-haiku"
        input_price_m: 3
        output_price_m: 15
        cache_price_m: 0.3
"""

CONFIG_WITHOUT_LIMITS = """\
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
    models:
      - name: "{upstream_model}"
        alias: "test-haiku"
"""

CONFIG_MGMT_ENABLED = """\
host: "{host}"
port: {port}
debug: true
usage-statistics-enabled: true
usage-db: {usage_db}
api-keys:
  - "{api_key}"
remote-management:
  allow-remote: false
  secret-key: "test-mgmt-key"
  disable-config-api: false
openai-compatibility:
  - name: "test-upstream"
    base-url: "{upstream_url}"
    api-key-entries:
      - api-key: "{upstream_key}"
    models:
      - name: "{upstream_model}"
        alias: "test-haiku"
"""

CONFIG_MGMT_DISABLED = """\
host: "{host}"
port: {port}
debug: true
usage-statistics-enabled: true
usage-db: {usage_db}
api-keys:
  - "{api_key}"
remote-management:
  allow-remote: false
  secret-key: "test-mgmt-key"
  disable-config-api: true
openai-compatibility:
  - name: "test-upstream"
    base-url: "{upstream_url}"
    api-key-entries:
      - api-key: "{upstream_key}"
    models:
      - name: "{upstream_model}"
        alias: "test-haiku"
"""

RELOAD_WAIT = 3  # seconds to wait for hot-reload (debounce + reload)


def _mgmt_headers():
    return {"Authorization": "Bearer test-mgmt-key"}


def test_removing_limits_clears_rate_limit(make_server):
    """After removing limits from config, requests that were 429 should succeed."""
    srv = make_server(CONFIG_WITH_LIMITS)
    srv.start()

    # Hit rate limit
    got_429 = False
    for _ in range(20):
        status, _ = srv.chat_completions("test-haiku")
        if status == 429:
            got_429 = True
            break
    assert got_429, "Should hit rate limit with limits configured"

    # Remove limits via hot-reload
    srv.write_config(CONFIG_WITHOUT_LIMITS)
    time.sleep(RELOAD_WAIT)

    # Request should now succeed
    status, _ = srv.chat_completions("test-haiku")
    assert status == 200, f"Expected 200 after removing limits, got {status}"


def test_toggling_disable_config_api(make_server):
    """Toggling disable-config-api on blocks writes; toggling off restores them."""
    srv = make_server(CONFIG_MGMT_ENABLED)
    srv.start()

    # With disable-config-api=false, PUT should not return 403
    status, _, body = srv.request(
        "/v0/management/api-keys",
        method="PUT",
        body=json.dumps({"value": "new-key"}).encode(),
        headers={**_mgmt_headers(), "Content-Type": "application/json"},
    )
    assert status != 403 or body.get("error") != "config API is disabled", \
        "PUT should not be blocked by disable-config-api when false"

    # Toggle disable-config-api on
    srv.write_config(CONFIG_MGMT_DISABLED)
    time.sleep(RELOAD_WAIT)

    status, _, body = srv.request(
        "/v0/management/api-keys",
        method="PUT",
        body=json.dumps({"value": "new-key"}).encode(),
        headers={**_mgmt_headers(), "Content-Type": "application/json"},
    )
    assert status == 403
    assert body.get("error") == "config API is disabled"

    # Toggle back off
    srv.write_config(CONFIG_MGMT_ENABLED)
    time.sleep(RELOAD_WAIT)

    status, _, body = srv.request(
        "/v0/management/api-keys",
        method="PUT",
        body=json.dumps({"value": "new-key"}).encode(),
        headers={**_mgmt_headers(), "Content-Type": "application/json"},
    )
    assert status != 403 or body.get("error") != "config API is disabled", \
        "PUT should not be blocked after re-enabling config API"
