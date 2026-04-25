"""Management API integration tests: disable-config-api blocks writes with 403."""

import json

DISABLE_CONFIG_API_TEMPLATE = """\
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


def _mgmt_headers(srv):
    return {"Authorization": f"Bearer test-mgmt-key"}


def test_get_config_succeeds(make_server):
    """GET /v0/management/config returns 200 when disable-config-api is true."""
    srv = make_server(DISABLE_CONFIG_API_TEMPLATE)
    srv.start()

    status, _, body = srv.request(
        "/v0/management/config", headers=_mgmt_headers(srv),
    )
    assert status == 200


def test_put_config_blocked(make_server):
    """PUT /v0/management/config.yaml returns 403 when disable-config-api is true."""
    srv = make_server(DISABLE_CONFIG_API_TEMPLATE)
    srv.start()

    status, _, body = srv.request(
        "/v0/management/config.yaml",
        method="PUT",
        body=b"host: 127.0.0.1",
        headers={**_mgmt_headers(srv), "Content-Type": "application/yaml"},
    )
    assert status == 403
    assert body.get("error") == "config API is disabled"


def test_put_api_keys_blocked(make_server):
    """PUT /v0/management/api-keys returns 403 when disable-config-api is true."""
    srv = make_server(DISABLE_CONFIG_API_TEMPLATE)
    srv.start()

    status, _, body = srv.request(
        "/v0/management/api-keys",
        method="PUT",
        body=json.dumps({"value": "new-key"}).encode(),
        headers={**_mgmt_headers(srv), "Content-Type": "application/json"},
    )
    assert status == 403


def test_no_management_key_returns_404(make_server):
    """Without a secret-key, management endpoints return 404."""
    # Use default CONFIG_TEMPLATE which has no remote-management section
    srv = make_server(None)
    srv.start()

    status, _, _ = srv.request(
        "/v0/management/config", headers=_mgmt_headers(srv),
    )
    assert status == 404
