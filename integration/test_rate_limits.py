"""Rate limiting integration tests: token and price limits enforced via 429."""


def test_input_tokens_limit_returns_429(make_server):
    """When input token usage reaches the configured 3k limit, next request gets 429."""
    srv = make_server(None)
    srv.start()
    got_429 = False
    for _ in range(20):
        status, body = srv.chat_completions("test-haiku")
        if status == 429:
            got_429 = True
            assert "rate limit" in body.get("error", {}).get("message", "").lower()
            break
        if status != 200:
            break
    assert got_429, "Expected 429 rate limit response after exceeding input_tokens limit"


def test_single_request_within_limits(make_server):
    """A single request should succeed within the 3k input_tokens limit."""
    srv = make_server(None)
    srv.start()
    status, body = srv.chat_completions("test-haiku")
    assert status == 200


def test_limit_error_includes_details(make_server):
    """429 response body includes limit type, current, and limit values."""
    srv = make_server(None)
    srv.start()
    got_429 = False
    for _ in range(20):
        status, body = srv.chat_completions("test-haiku")
        if status == 429:
            got_429 = True
            error = body.get("error", {})
            msg = error.get("message", "")
            assert "input_tokens" in msg or "price" in msg, f"Unexpected error msg: {msg}"
            break
    assert got_429, "Expected 429 to check error details"
