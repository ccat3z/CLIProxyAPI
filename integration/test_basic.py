"""Basic integration tests: server startup, successful requests, usage recording."""

import os
import sqlite3
import urllib.request


def test_server_starts_and_responds(make_server):
    """Server starts with usage-db config and responds to /v1/models."""
    srv = make_server(None)
    srv.start()
    req = urllib.request.Request(
        srv.base_url + "/v1/models",
        headers={"Authorization": f"Bearer {srv.API_KEY}"},
    )
    with urllib.request.urlopen(req, timeout=5) as resp:
        assert resp.status == 200


def test_chat_completions_success(make_server):
    """A chat completion request succeeds and returns a valid response."""
    srv = make_server(None)
    srv.start()
    status, body = srv.chat_completions("test-haiku")
    assert status == 200
    assert "choices" in body
    assert len(body["choices"]) > 0


def test_usage_recorded_to_sqlite(make_server):
    """After a successful request, usage data is written to the SQLite DB."""
    srv = make_server(None)
    srv.start()
    status, _ = srv.chat_completions("test-haiku")
    assert status == 200

    db_path = os.path.join(srv.usage_db_dir, "usage.db")
    conn = sqlite3.connect(db_path)
    try:
        rows = conn.execute("SELECT count(*) FROM usage").fetchone()
        assert rows[0] > 0, "Usage table should have at least one entry"

        row = conn.execute(
            "SELECT input_tokens, output_tokens, cost FROM usage LIMIT 1"
        ).fetchone()
        assert row[0] > 0, "input_tokens should be > 0"
        assert row[1] > 0, "output_tokens should be > 0"
        assert row[2] > 0.0, "cost should be > 0 (prices are configured)"
    finally:
        conn.close()


def test_chat_completions_invalid_api_key(make_server):
    """Request with wrong API key returns 401."""
    srv = make_server(None)
    srv.start()
    status, _ = srv.chat_completions("test-haiku", api_key="wrong-key")
    assert status == 401
