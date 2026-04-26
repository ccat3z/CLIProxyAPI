"""Dynamic config integration tests: hot-reload of config.yaml at runtime."""

import json
import time

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
