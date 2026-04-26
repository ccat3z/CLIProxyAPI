"""Persistence integration tests: rate limits survive server restarts."""

import os
import sqlite3


def test_limits_persist_across_restart(make_server):
    """After restarting the server, previously accumulated usage is still enforced."""
    srv = make_server(None)
    srv.start()

    # Send requests until rate limited
    got_429 = False
    total_input = 0
    for _ in range(10):
        status, _, body = srv.chat_completions("test-haiku")
        if status == 429:
            got_429 = True
            break
        if status != 200:
            break
        total_input += body.get("usage", {}).get("prompt_tokens", 0)
    assert got_429, f"Should hit rate limit before restart (accumulated {total_input})"

    # Restart server — usage DB should persist
    srv.restart()

    # First request after restart should still be rate limited
    status, _, body = srv.chat_completions("test-haiku")
    assert status == 429, f"Expected 429 after restart, got {status}: {body}"


def test_usage_db_file_exists(make_server):
    """The SQLite usage DB file is created on the filesystem."""
    srv = make_server(None)
    srv.start()

    srv.chat_completions("test-haiku")  # noqa: ignore returned tuple
    db_path = os.path.join(srv.usage_db_dir, "usage.db")
    assert os.path.exists(db_path), "usage.db should exist on disk"


def test_usage_records_persist_across_restart(make_server):
    """SQLite usage records survive restart — rows exist in DB after restart."""
    srv = make_server(None)
    srv.start()

    status, _, _ = srv.chat_completions("test-haiku")
    assert status == 200

    db_path = os.path.join(srv.usage_db_dir, "usage.db")

    # Read row count before restart
    conn = sqlite3.connect(db_path)
    count_before = conn.execute("SELECT COUNT(*) FROM usage").fetchone()[0]
    conn.close()
    assert count_before > 0, "Expected at least one usage row before restart"

    srv.restart()

    # DB rows should still exist after restart
    conn = sqlite3.connect(db_path)
    count_after = conn.execute("SELECT COUNT(*) FROM usage").fetchone()[0]
    conn.close()
    assert count_after == count_before, (
        f"Usage rows changed after restart: before={count_before}, after={count_after}"
    )
