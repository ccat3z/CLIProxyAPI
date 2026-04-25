"""Persistence integration tests: rate limits survive server restarts."""

import os


def test_limits_persist_across_restart(make_server):
    """After restarting the server, previously accumulated usage is still enforced."""
    srv = make_server(None)
    srv.start()

    # Send requests until rate limited
    got_429 = False
    for _ in range(20):
        status, _ = srv.chat_completions("test-haiku")
        if status == 429:
            got_429 = True
            break
    assert got_429, "Should hit rate limit before restart"

    # Restart server — usage DB should persist
    srv.restart()

    # First request after restart should still be rate limited
    status, body = srv.chat_completions("test-haiku")
    assert status == 429, f"Expected 429 after restart, got {status}: {body}"


def test_usage_db_file_exists(make_server):
    """The SQLite usage DB file is created on the filesystem."""
    srv = make_server(None)
    srv.start()

    srv.chat_completions("test-haiku")
    db_path = os.path.join(srv.usage_db_dir, "usage.db")
    assert os.path.exists(db_path), "usage.db should exist on disk"
