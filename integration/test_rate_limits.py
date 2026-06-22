"""Rate limiting integration tests: token and price limits enforced via 429."""

import random
import string
import time


def _make_long_prompt():
    nonce = ''.join(random.choices(string.ascii_lowercase + string.digits, k=16))
    return f"""
      [ref:{nonce}]
      Summary this in 50 words:

      Lorem ipsum dolor sit amet, consectetur adipiscing elit. Praesent et fringilla purus. Praesent id sapien vehicula elit pulvinar maximus sed id nunc. Aliquam tristique, ante a dignissim auctor, nisi massa sollicitudin massa, molestie maximus erat justo vel sem. Aenean mattis maximus enim et hendrerit. Donec eleifend, sapien a convallis porta, diam justo feugiat tortor, vulputate ullamcorper felis ante eget sapien. Quisque fringilla ut metus efficitur scelerisque. Nunc laoreet vestibulum nisi sed luctus. Quisque varius, est a auctor laoreet, metus mauris feugiat urna, vitae sagittis quam massa in est. Etiam sodales suscipit tortor, nec tempus turpis facilisis et.
      Nulla libero lacus, consectetur et justo vitae, pharetra ornare nisi. Morbi commodo nunc et facilisis mattis. Morbi in molestie quam. Curabitur nisi risus, luctus in consectetur vel, convallis eget lorem. Etiam aliquet vitae libero in condimentum. Donec tempor, urna sed hendrerit viverra, enim felis malesuada eros, sed dictum felis risus vitae justo. Donec tristique enim ut commodo rhoncus. Nullam mollis justo non ipsum scelerisque, sollicitudin euismod neque semper.
      Ut porta congue ex, sit amet ultricies elit consectetur non. Aliquam dolor nisi, mollis non elit a, aliquam bibendum quam. Maecenas quis felis quam. Morbi eu tortor vitae lacus sodales aliquam ut a libero. Donec elit urna, ultrices sed molestie ac, sagittis et justo. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Nulla facilisi.
      Sed interdum felis at sem pellentesque, volutpat dictum mi condimentum. Quisque non laoreet justo. Nunc dictum velit vel orci aliquam pellentesque. Mauris at tortor in augue mollis eleifend. Aliquam nec nibh non augue tincidunt congue. Quisque vel semper nisi. Vivamus sagittis erat nisi, ut placerat libero tempus sed. Vestibulum blandit a lorem ac posuere.
      Fusce vitae varius lorem. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Integer ac dui nec dolor condimentum vulputate nec porttitor ante. Sed posuere interdum arcu, vel iaculis mauris vestibulum sed. Sed et nibh tincidunt, scelerisque lorem nec, pretium turpis. Aliquam vehicula ut dui id efficitur. Maecenas massa dolor, volutpat quis libero ut, euismod aliquet mauris. In dignissim, lorem id dignissim suscipit, dolor ligula cursus ex, at maximus arcu nunc quis mauris. Phasellus sit amet enim lacus. Aliquam sed semper lacus, sit amet hendrerit felis. Praesent gravida placerat mauris vel vehicula.
      Ut blandit lorem eu lacus faucibus bibendum. Vestibulum magna elit, placerat sit amet convallis et, convallis ut orci. Mauris eu scelerisque mauris. Fusce sed risus a purus facilisis vestibulum eu ut justo. Maecenas nisl augue, porttitor quis sem sed, scelerisque viverra sapien. Vestibulum fringilla magna non ipsum sollicitudin condimentum. Sed ultricies orci nunc, sit amet accumsan ipsum porta in. Vestibulum fringilla pharetra ipsum quis rhoncus. Morbi elementum tincidunt ligula sed tincidunt. Donec commodo commodo dolor eu ullamcorper.
      Morbi molestie quam quis sem malesuada, ut congue ligula sodales. Fusce rhoncus venenatis vestibulum. Suspendisse non nisl elit. Vivamus dictum, nibh eget egestas auctor, mauris erat porttitor leo, consequat gravida sapien ligula eu mi. Aenean eu velit pulvinar, mollis libero at, ultrices metus. Donec hendrerit orci semper, venenatis magna eget, aliquam libero. Sed ac nisi ac mi ultricies ornare maximus aliquam nunc. Praesent condimentum finibus urna id dictum. Nam vel diam eu felis sodales tincidunt. Sed condimentum, ante eget porttitor hendrerit, nisi arcu tincidunt neque, vitae rhoncus ipsum eros in justo. Aliquam erat volutpat. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas.
      Praesent ut massa a enim iaculis lacinia sed pulvinar libero. Etiam viverra, purus nec pharetra consequat, enim ipsum vehicula erat, ac elementum metus lacus id risus. Curabitur ligula mauris, vulputate non ullamcorper imperdiet, bibendum vitae neque. Integer tincidunt aliquet luctus. Vestibulum leo nulla, consequat vitae nibh eget, semper eleifend urna. Ut ultricies felis lorem, elementum molestie est egestas nec. Sed nec magna ut magna sollicitudin mattis eu eget risus. Aenean feugiat nisl ut tellus viverra, id pulvinar arcu auctor. Pellentesque est ipsum, sagittis vel massa ut, elementum consectetur augue. Interdum et malesuada fames ac ante ipsum primis in faucibus. Donec commodo, magna ut tempor pulvinar, elit ligula posuere nibh, quis auctor lacus sapien ac mi. Proin lacinia, risus eu porta maximus, velit felis dictum magna, eget viverra nulla ex a lacus. Quisque imperdiet, neque eget viverra cursus, orci diam luctus odio, eu pretium augue mi non ex. Nulla posuere volutpat felis quis porttitor. Aenean consequat, libero in convallis luctus, nisi odio porta ante, ac euismod sem mi non magna.
      Etiam finibus quis erat id dapibus. Ut enim justo, luctus nec ex at, tempor dignissim mauris. Suspendisse nec molestie arcu. Aenean ante libero, semper hendrerit metus eu, vestibulum tempus nisi. Mauris rutrum pulvinar porta. Ut venenatis elementum ante. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Donec blandit ex eget orci lobortis, a ultrices dolor faucibus. Donec sit amet metus fermentum, rhoncus sem id, consequat risus. Nam eu sapien tempor justo ornare aliquam.
      Aenean massa dolor, placerat in augue et, consequat pharetra nunc. Etiam convallis tempor ipsum. Praesent vehicula rhoncus rhoncus. Morbi ligula magna, mollis sit amet finibus sit amet, auctor ac enim. Nunc scelerisque, odio non congue lobortis, dui risus faucibus eros, sit amet consequat nisl purus vel lorem. Proin eget varius enim, condimentum pretium dui. Interdum et malesuada fames ac ante ipsum primis in faucibus. Sed sed gravida justo, a consequat risus. Aliquam condimentum odio nibh, ullamcorper rutrum odio auctor ac. Cras sem orci, posuere id elit a, scelerisque varius diam. Mauris malesuada diam in odio sodales rutrum. Phasellus tempor orci id elit congue, quis aliquet orci accumsan. Praesent a libero interdum, luctus lectus at, consequat urna. Vivamus arcu lorem, rhoncus gravida gravida dictum, interdum eget nisl. Vivamus aliquam mi in malesuada tempus.
      Vestibulum varius dapibus lobortis. In mattis accumsan neque, vitae ornare orci. Vivamus in molestie risus. Mauris ipsum metus, luctus molestie blandit a, rutrum vel erat. Vestibulum eget urna commodo, molestie urna non, rutrum nisi. Maecenas condimentum vitae nisl vel porttitor. In lacinia, nibh et fermentum gravida, sem mi iaculis velit, quis pharetra leo lorem eget lectus. Pellentesque condimentum vel odio ac tincidunt. Nam risus urna, hendrerit mattis sagittis.
    """


_MULTI_MODEL_TEMPLATE = """\
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
            input_tokens: 3k
            models: ["{upstream_model}", "{upstream_model_2}"]
    models:
      - name: "{upstream_model}"
        alias: "test-haiku"
        input_price_m: 3
        output_price_m: 15
        cache_price_m: 0.3
      - name: "{upstream_model_2}"
        alias: "test-sonnet"
        input_price_m: 3
        output_price_m: 15
        cache_price_m: 0.3
"""

_WILDCARD_TEMPLATE = """\
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
            input_tokens: 3k
            models: []
    models:
      - name: "{upstream_model}"
        alias: "test-haiku"
        input_price_m: 3
        output_price_m: 15
        cache_price_m: 0.3
"""

_PRICE_LIMIT_CONFIG = """\
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

_1K_LIMIT_CONFIG = """\
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

_TWO_KEY_CONFIG = """\
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

_CONFIG_WITHOUT_LIMITS = """\
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

RELOAD_WAIT = 3  # seconds to wait for hot-reload (debounce + reload)


def _drain_until_429(srv, model, api_key=None):
    """Send chat requests until 429, tracking tokens from responses."""
    total_input = 0
    for _ in range(10):
        status, _, body = srv.chat_completions(model, api_key=api_key, prompt=_make_long_prompt())
        if status == 429:
            return status, body, total_input
        if status != 200:
            return status, body, total_input
        total_input += body.get("usage", {}).get("prompt_tokens", 0)
    return status, body, total_input


def test_input_tokens_limit_returns_429(make_server):
    """When input token usage reaches the configured 3k limit, next request gets 429."""
    srv = make_server(None, model="small")
    srv.start()
    status, body, total_input = _drain_until_429(srv, "test-haiku")
    assert status == 429, (
        f"Expected 429 after exceeding input_tokens limit (accumulated {total_input})"
    )
    assert "rate limit" in body.get("error", {}).get("message", "").lower()


def test_limit_error_includes_details(make_server):
    """429 response body includes limit type, current, and limit values."""
    srv = make_server(None, model="small")
    srv.start()
    status, body, _ = _drain_until_429(srv, "test-haiku")
    assert status == 429, "Expected 429 to check error details"
    error = body.get("error", {})
    msg = error.get("message", "")
    assert "input_tokens" in msg or "price" in msg, f"Unexpected error msg: {msg}"


def test_shared_window_models_trigger_429(make_server):
    """Models in the same limit window share usage: combined tokens trigger 429."""
    srv = make_server(_MULTI_MODEL_TEMPLATE, model="small")
    srv.start()
    total_input = 0
    got_429 = False
    for i in range(10):
        model = "test-haiku" if i % 2 == 0 else "test-sonnet"
        status, _, body = srv.chat_completions(model, prompt=_make_long_prompt())
        if status == 429:
            got_429 = True
            assert "rate limit" in body.get("error", {}).get("message", "").lower()
            break
        if status != 200:
            break
        total_input += body.get("usage", {}).get("prompt_tokens", 0)
    assert got_429, (
        f"Expected 429 when combined usage exceeds limit (accumulated {total_input})"
    )


def test_wildcard_limit_trigger_429(make_server):
    """Wildcard (empty models) limit applies to all models for the authID."""
    srv = make_server(_WILDCARD_TEMPLATE, model="small")
    srv.start()
    status, body, total_input = _drain_until_429(srv, "test-haiku")
    assert status == 429, (
        f"Expected 429 when wildcard limit exceeded (accumulated {total_input})"
    )
    assert "rate limit" in body.get("error", {}).get("message", "").lower()


def test_price_limit_returns_429(make_server):
    """When accumulated cost exceeds the price limit, 429 is returned."""
    srv = make_server(_PRICE_LIMIT_CONFIG, model="small")
    srv.start()

    total_input = 0
    for _ in range(10):
        status, _, body = srv.chat_completions("test-haiku", prompt=_make_long_prompt())
        if status == 429:
            msg = body.get("error", {}).get("message", "")
            assert "price" in msg, f"Expected price in error, got: {msg}"
            return
        if status != 200:
            break
        total_input += body.get("usage", {}).get("prompt_tokens", 0)
    raise AssertionError(
        f"Should hit price limit with $0.01 cap (accumulated {total_input} input tokens)"
    )


def test_429_has_no_retry_after_header(make_server):
    """429 responses must not include a Retry-After header."""
    srv = make_server(_1K_LIMIT_CONFIG, model="small")
    srv.start()

    total_input = 0
    for _ in range(10):
        status, headers, body = srv.chat_completions("test-haiku", prompt=_make_long_prompt())
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


def test_different_keys_share_same_limit_pool(make_server):
    """Two API keys under the same upstream key share the limit pool."""
    srv = make_server(_TWO_KEY_CONFIG, model="small")
    srv.start()

    # Exhaust limit with key-low
    got_429 = False
    total_input = 0
    for _ in range(10):
        status, _, body = srv.chat_completions("test-haiku", api_key="key-low", prompt=_make_long_prompt())
        if status == 429:
            got_429 = True
            break
        if status != 200:
            break
        total_input += body.get("usage", {}).get("prompt_tokens", 0)
    assert got_429, f"key-low should hit rate limit (accumulated {total_input})"

    # key-high should also be limited (same upstream pool)
    status, _, _ = srv.chat_completions("test-haiku", api_key="key-high")
    assert status == 429, "key-high should also be limited (shared upstream pool)"


def test_limits_persist_across_restart(make_server):
    """After restarting the server, previously accumulated usage is still enforced."""
    srv = make_server(None, model="small")
    srv.start()

    # Send requests until rate limited
    got_429 = False
    total_input = 0
    for _ in range(10):
        status, _, body = srv.chat_completions("test-haiku", prompt=_make_long_prompt())
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


def test_removing_limits_clears_rate_limit(make_server):
    """After removing limits from config, requests that were 429 should succeed."""
    srv = make_server(_1K_LIMIT_CONFIG, model="small")
    srv.start()

    # Hit rate limit
    got_429 = False
    total_input = 0
    for _ in range(10):
        status, _, body = srv.chat_completions("test-haiku", prompt=_make_long_prompt())
        if status == 429:
            got_429 = True
            break
        if status != 200:
            break
        total_input += body.get("usage", {}).get("prompt_tokens", 0)
    assert got_429, f"Should hit rate limit with limits configured (accumulated {total_input})"

    # Remove limits via hot-reload
    srv.write_config(_CONFIG_WITHOUT_LIMITS)
    time.sleep(RELOAD_WAIT)

    # Request should now succeed
    status, _, _ = srv.chat_completions("test-haiku")
    assert status == 200, f"Expected 200 after removing limits, got {status}"
