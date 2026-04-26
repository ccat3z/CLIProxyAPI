"""Usage API integration tests: /v0/management/usage endpoint with SQLite backend."""

import os
import time

CONFIG_WITH_MGMT = """\
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
openai-compatibility:
  - name: "test-upstream"
    base-url: "{upstream_url}"
    api-key-entries:
      - api-key: "{upstream_key}"
        limits:
          - window: 1h
            input_tokens: 20k
            models: ["{upstream_model}"]
    models:
      - name: "{upstream_model}"
        alias: "test-haiku"
        input_price_m: 3
        output_price_m: 15
        cache_price_m: 0.3
"""

CONFIG_WITH_REQUEST_LOG = """\
host: "{host}"
port: {port}
debug: true
request-log: true
usage-statistics-enabled: true
usage-db: {usage_db}
api-keys:
  - "{api_key}"
remote-management:
  allow-remote: false
  secret-key: "test-mgmt-key"
openai-compatibility:
  - name: "test-upstream"
    base-url: "{upstream_url}"
    api-key-entries:
      - api-key: "{upstream_key}"
        limits:
          - window: 1h
            input_tokens: 20k
            models: ["{upstream_model}"]
    models:
      - name: "{upstream_model}"
        alias: "test-haiku"
        input_price_m: 3
        output_price_m: 15
        cache_price_m: 0.3
"""

MGMT_PATH = "/v0/management/usage"
MGMT_SECRET = "test-mgmt-key"


def _mgmt_headers():
    return {"Authorization": f"Bearer {MGMT_SECRET}"}


def _fetch_usage(srv):
    """Helper: wait for usage to be persisted, then fetch usage data."""
    time.sleep(1)
    status, _, body = srv.request(MGMT_PATH, headers=_mgmt_headers())
    assert status == 200, f"Usage API returned {status}: {body}"
    return body


def test_usage_matches_chat_completions_response(make_server):
    """Token counts in usage API match those returned by chat completions across multiple rounds."""
    srv = make_server(CONFIG_WITH_MGMT)
    srv.start()

    num_rounds = 3
    chat_usages = []
    for i in range(num_rounds):
        status, _, chat_body = srv.chat_completions("test-haiku")
        assert status == 200, f"Chat round {i+1} returned {status}"
        chat_usage = chat_body.get("usage", {})
        assert "prompt_tokens" in chat_usage, f"Round {i+1} response missing usage.prompt_tokens"
        chat_usages.append(chat_usage)

    mgmt_body = _fetch_usage(srv)
    usage = mgmt_body["usage"]

    # Sum across all chat rounds
    sum_prompt = sum(u["prompt_tokens"] for u in chat_usages)
    sum_completion = sum(u["completion_tokens"] for u in chat_usages)
    sum_total = sum(u["total_tokens"] for u in chat_usages)
    sum_cached = sum(u.get("prompt_tokens_details", {}).get("cached_tokens", 0) for u in chat_usages)

    # Aggregate detail tokens across all entries
    detail_input = 0
    detail_output = 0
    detail_total = 0
    detail_cached = 0
    detail_count = 0
    for _ak, api_data in usage["apis"].items():
        for _mk, model_data in api_data["models"].items():
            for detail in model_data["details"]:
                tokens = detail["tokens"]
                detail_input += tokens["input_tokens"]
                detail_output += tokens["output_tokens"]
                detail_total += tokens["total_tokens"]
                detail_cached += tokens["cached_tokens"]
                detail_count += 1

    assert detail_count == num_rounds, \
        f"Expected {num_rounds} detail entries, got {detail_count}"

    # Per-field sums must match
    assert detail_input == sum_prompt, \
        f"input_tokens sum {detail_input} != prompt_tokens sum {sum_prompt}"
    assert detail_output == sum_completion, \
        f"output_tokens sum {detail_output} != completion_tokens sum {sum_completion}"
    assert detail_total == sum_total, \
        f"total_tokens sum {detail_total} != chat total_tokens sum {sum_total}"
    assert detail_cached == sum_cached, \
        f"cached_tokens sum {detail_cached} != chat cached_tokens sum {sum_cached}"

    # Top-level total_tokens must match
    assert usage["total_tokens"] == sum_total, \
        f"Usage API total_tokens {usage['total_tokens']} != chat total_tokens sum {sum_total}"

    # Per-detail: each entry must match its corresponding chat round
    all_details = []
    for _ak, api_data in usage["apis"].items():
        for _mk, model_data in api_data["models"].items():
            all_details.extend(model_data["details"])

    # Sort by timestamp to match order
    all_details.sort(key=lambda d: d["timestamp"])
    for i, (detail, chat_usage) in enumerate(zip(all_details, chat_usages)):
        tokens = detail["tokens"]
        assert tokens["input_tokens"] == chat_usage["prompt_tokens"], \
            f"Round {i+1}: input_tokens {tokens['input_tokens']} != prompt_tokens {chat_usage['prompt_tokens']}"
        assert tokens["output_tokens"] == chat_usage["completion_tokens"], \
            f"Round {i+1}: output_tokens {tokens['output_tokens']} != completion_tokens {chat_usage['completion_tokens']}"
        assert tokens["total_tokens"] == chat_usage["total_tokens"], \
            f"Round {i+1}: total_tokens {tokens['total_tokens']} != chat total_tokens {chat_usage['total_tokens']}"
        chat_cached = chat_usage.get("prompt_tokens_details", {}).get("cached_tokens", 0)
        assert tokens["cached_tokens"] == chat_cached, \
            f"Round {i+1}: cached_tokens {tokens['cached_tokens']} != chat cached_tokens {chat_cached}"

    # Aggregated cost must equal sum of per-detail expected costs
    sum_cost = 0.0
    for detail in all_details:
        tokens = detail["tokens"]
        non_cached = tokens["input_tokens"] - tokens["cached_tokens"]
        expected = (non_cached * 3.0 + tokens["cached_tokens"] * 0.3 +
                    tokens["output_tokens"] * 15.0) / 1_000_000
        assert abs(detail["cost"] - expected) < 0.000001, \
            f"Detail cost {detail['cost']} != expected {expected}"
        sum_cost += detail["cost"]

    cost_by_day_sum = sum(usage["cost_by_day"].values())
    assert abs(sum_cost - cost_by_day_sum) < 0.0001, \
        f"Detail cost sum {sum_cost} != cost_by_day sum {cost_by_day_sum}"


def test_usage_api_response_structure(make_server):
    """The usage response has the expected top-level and nested structure."""
    srv = make_server(CONFIG_WITH_MGMT)
    srv.start()

    status, _, _ = srv.chat_completions("test-haiku")
    assert status == 200

    body = _fetch_usage(srv)

    # Top-level keys
    assert "usage" in body, "Response should have 'usage' key"
    assert "failed_requests" in body, "Response should have 'failed_requests' key"
    assert isinstance(body["failed_requests"], int)

    usage = body["usage"]

    # Top-level usage fields
    for key in ("total_requests", "success_count", "failure_count", "total_tokens",
                "apis", "requests_by_day", "requests_by_hour",
                "tokens_by_day", "tokens_by_hour",
                "cost_by_day", "cost_by_hour"):
        assert key in usage, f"usage should include '{key}'"

    assert isinstance(usage["total_requests"], int)
    assert usage["total_requests"] >= 1
    assert isinstance(usage["success_count"], int)
    assert usage["success_count"] >= 1
    assert isinstance(usage["failure_count"], int)
    assert isinstance(usage["total_tokens"], int)
    assert usage["total_tokens"] > 0

    # Time-bucketed aggregations
    for bucket_key in ("requests_by_day", "tokens_by_day", "cost_by_day"):
        assert isinstance(usage[bucket_key], dict)
    for bucket_key in ("requests_by_hour", "tokens_by_hour", "cost_by_hour"):
        assert isinstance(usage[bucket_key], dict)

    # APIs section
    apis = usage["apis"]
    assert isinstance(apis, dict)
    assert len(apis) > 0, "Should have at least one API entry"

    for api_key, api_data in apis.items():
        assert isinstance(api_key, str)
        assert "total_requests" in api_data
        assert "total_tokens" in api_data
        assert "models" in api_data
        assert isinstance(api_data["total_requests"], int)
        assert isinstance(api_data["total_tokens"], int)
        assert isinstance(api_data["models"], dict)
        assert api_data["total_requests"] >= 1
        assert api_data["total_tokens"] > 0

        for model_name, model_data in api_data["models"].items():
            assert isinstance(model_name, str)
            assert "total_requests" in model_data
            assert "total_tokens" in model_data
            assert "details" in model_data
            assert isinstance(model_data["details"], list)
            assert len(model_data["details"]) >= 1


def test_usage_api_detail_entry_fields(make_server):
    """Each detail entry has the expected fields with correct types."""
    srv = make_server(CONFIG_WITH_MGMT)
    srv.start()

    status, _, _ = srv.chat_completions("test-haiku")
    assert status == 200

    body = _fetch_usage(srv)
    usage = body["usage"]

    # Get first detail entry
    detail = None
    for _ak, api_data in usage["apis"].items():
        for _mk, model_data in api_data["models"].items():
            if model_data["details"]:
                detail = model_data["details"][0]
                break
        if detail:
            break

    assert detail is not None, "Should have at least one detail entry"

    # Required string fields
    assert "timestamp" in detail
    assert isinstance(detail["timestamp"], str)
    assert "source" in detail
    assert isinstance(detail["source"], str)
    assert "auth_index" in detail
    assert isinstance(detail["auth_index"], str)
    assert "request_id" in detail
    assert isinstance(detail["request_id"], str)

    # Required numeric fields
    assert "latency_ms" in detail
    assert isinstance(detail["latency_ms"], int)
    assert detail["latency_ms"] >= 0

    assert "cost" in detail
    assert isinstance(detail["cost"], (int, float))
    assert detail["cost"] > 0, "Cost should be > 0 for successful priced request"

    assert "failed" in detail
    assert isinstance(detail["failed"], bool)
    assert detail["failed"] is False, "Successful request should not be marked failed"

    # Token sub-object
    assert "tokens" in detail
    tokens = detail["tokens"]
    for key in ("input_tokens", "output_tokens", "reasoning_tokens",
                "cached_tokens", "total_tokens"):
        assert key in tokens, f"tokens should include '{key}'"
        assert isinstance(tokens[key], int)

    assert tokens["input_tokens"] > 0
    assert tokens["output_tokens"] > 0
    assert tokens["total_tokens"] > 0


def test_usage_api_cost_values(make_server):
    """Cost calculations are consistent with configured prices."""
    srv = make_server(CONFIG_WITH_MGMT)
    srv.start()

    status, _, _ = srv.chat_completions("test-haiku")
    assert status == 200

    body = _fetch_usage(srv)
    usage = body["usage"]

    # cost_by_day should have at least one entry with positive cost
    cost_by_day = usage["cost_by_day"]
    assert len(cost_by_day) > 0, "cost_by_day should have entries"
    total_cost_day = sum(cost_by_day.values())
    assert total_cost_day > 0, "Total cost_by_day should be > 0"

    # cost_by_hour should have at least one entry with positive cost
    cost_by_hour = usage["cost_by_hour"]
    assert len(cost_by_hour) > 0, "cost_by_hour should have entries"
    total_cost_hour = sum(cost_by_hour.values())
    assert total_cost_hour > 0, "Total cost_by_hour should be > 0"

    # Sum of detail costs should equal sum of cost_by_day values
    detail_cost_sum = 0.0
    for _ak, api_data in usage["apis"].items():
        for _mk, model_data in api_data["models"].items():
            for detail in model_data["details"]:
                detail_cost_sum += detail["cost"]
    assert abs(detail_cost_sum - total_cost_day) < 0.0001, \
        f"Detail cost sum ({detail_cost_sum}) != cost_by_day sum ({total_cost_day})"

    # Detail cost should be reasonable: (input*3 + output*15) / 1M
    # With no cached tokens, cost = input*3/1M + output*15/1M
    detail = None
    for _ak, api_data in usage["apis"].items():
        for _mk, model_data in api_data["models"].items():
            if model_data["details"]:
                detail = model_data["details"][0]
                break
        if detail:
            break

    tokens = detail["tokens"]
    non_cached = tokens["input_tokens"] - tokens["cached_tokens"]
    expected_cost = (non_cached * 3.0 + tokens["cached_tokens"] * 0.3 +
                     tokens["output_tokens"] * 15.0) / 1_000_000
    assert abs(detail["cost"] - expected_cost) < 0.000001, \
        f"Detail cost {detail['cost']} != expected {expected_cost}"


def test_usage_api_request_id_with_request_log(make_server):
    """When request-log is enabled, request_id is a non-empty string."""
    srv = make_server(CONFIG_WITH_REQUEST_LOG)
    srv.start()

    status, _, _ = srv.chat_completions("test-haiku")
    assert status == 200

    body = _fetch_usage(srv)
    usage = body["usage"]

    # Find a detail entry
    detail = None
    for _ak, api_data in usage["apis"].items():
        for _mk, model_data in api_data["models"].items():
            if model_data["details"]:
                detail = model_data["details"][0]
                break
        if detail:
            break

    assert detail is not None, "Should have at least one detail entry"
    assert "request_id" in detail
    assert isinstance(detail["request_id"], str)
    assert len(detail["request_id"]) > 0, \
        "request_id should be non-empty when request-log is enabled"


def test_usage_api_window_param(make_server):
    """The ?window=N parameter limits the query time range."""
    srv = make_server(CONFIG_WITH_MGMT)
    srv.start()

    status, _, _ = srv.chat_completions("test-haiku")
    assert status == 200

    time.sleep(1)

    # With window=1 (1 hour), should find recent data
    status, _, body = srv.request(
        MGMT_PATH + "?window=1", headers=_mgmt_headers()
    )
    assert status == 200
    usage = body.get("usage", {})
    assert usage.get("total_requests", 0) >= 1

    # With window=0 (invalid), should default to 24h
    status, _, body = srv.request(
        MGMT_PATH + "?window=0", headers=_mgmt_headers()
    )
    assert status == 200

    # With window=abc (invalid), should default to 24h
    status, _, body = srv.request(
        MGMT_PATH + "?window=abc", headers=_mgmt_headers()
    )
    assert status == 200


def test_usage_api_counters_consistent(make_server):
    """Top-level counters are consistent with aggregated data."""
    srv = make_server(CONFIG_WITH_MGMT)
    srv.start()

    status, _, _ = srv.chat_completions("test-haiku")
    assert status == 200

    body = _fetch_usage(srv)
    usage = body["usage"]

    # total_requests = success_count + failure_count
    assert usage["total_requests"] == usage["success_count"] + usage["failure_count"]

    # failed_requests at top level matches failure_count in usage
    assert body["failed_requests"] == usage["failure_count"]

    # Sum of API total_requests equals top-level total_requests
    api_request_sum = sum(api["total_requests"] for api in usage["apis"].values())
    assert api_request_sum == usage["total_requests"]

    # Sum of API total_tokens equals top-level total_tokens
    api_token_sum = sum(api["total_tokens"] for api in usage["apis"].values())
    assert api_token_sum == usage["total_tokens"]

    # Sum of requests_by_day equals total_requests
    assert sum(usage["requests_by_day"].values()) == usage["total_requests"]

    # Sum of requests_by_hour equals total_requests
    assert sum(usage["requests_by_hour"].values()) == usage["total_requests"]

    # Sum of tokens_by_day equals total_tokens
    assert sum(usage["tokens_by_day"].values()) == usage["total_tokens"]


def test_usage_api_limits_field_structure(make_server):
    """The 'limits' field in the usage response has the expected structure with config and current usage."""
    srv = make_server(CONFIG_WITH_MGMT)
    srv.start()

    status, _, _ = srv.chat_completions("test-haiku")
    assert status == 200

    body = _fetch_usage(srv)

    assert "limits" in body, "Response should have 'limits' key"
    limits = body["limits"]
    assert isinstance(limits, list), "limits should be a list"
    assert len(limits) > 0, "limits should have at least one entry (limits are configured)"

    entry = limits[0]
    assert "source" in entry, "limit entry should have 'source'"
    assert isinstance(entry["source"], str)
    assert "auth_index" in entry, "limit entry should have 'auth_index'"
    assert isinstance(entry["auth_index"], str)

    assert "config" in entry, "limit entry should have 'config'"
    config = entry["config"]
    assert "window" in config and isinstance(config["window"], int)
    assert config["window"] == 3600, f"Expected 3600s (1h) window, got {config['window']}"
    assert "models" in config and isinstance(config["models"], list)
    assert len(config["models"]) > 0
    assert "input_tokens" in config and isinstance(config["input_tokens"], int)
    assert config["input_tokens"] == 20000, f"Expected 20000 input_tokens, got {config['input_tokens']}"
    assert "output_tokens" in config and isinstance(config["output_tokens"], int)
    assert "cache_tokens" in config and isinstance(config["cache_tokens"], int)
    assert "price" in config and isinstance(config["price"], (int, float))

    assert "current" in entry, "limit entry should have 'current'"
    current = entry["current"]
    assert "input_tokens" in current and isinstance(current["input_tokens"], int)
    assert "output_tokens" in current and isinstance(current["output_tokens"], int)
    assert "cache_tokens" in current and isinstance(current["cache_tokens"], int)
    assert "price" in current and isinstance(current["price"], (int, float))


def test_usage_api_limits_current_reflects_usage(make_server):
    """Current usage in limits reflects actual token consumption from requests."""
    srv = make_server(CONFIG_WITH_MGMT)
    srv.start()

    # Make a single request
    status, _, chat_body = srv.chat_completions("test-haiku")
    assert status == 200

    body = _fetch_usage(srv)
    limits = body["limits"]
    assert len(limits) > 0

    # Limits are keyed by upstream model name; just use the first entry
    # since the test config only has one limit.
    entry = limits[0]

    current = entry["current"]
    chat_usage = chat_body.get("usage", {})
    assert chat_usage.get("prompt_tokens", 0) > 0, "Chat response should have prompt_tokens"

    assert current["input_tokens"] >= chat_usage["prompt_tokens"], \
        f"Current input_tokens {current['input_tokens']} should be >= chat prompt_tokens {chat_usage['prompt_tokens']}"
    assert current["output_tokens"] >= chat_usage.get("completion_tokens", 0), \
        f"Current output_tokens {current['output_tokens']} should be >= chat completion_tokens {chat_usage.get('completion_tokens', 0)}"


def test_usage_api_limits_multiple_rounds(make_server):
    """After multiple requests, current usage accumulates across the window."""
    srv = make_server(CONFIG_WITH_MGMT)
    srv.start()

    num_rounds = 3
    total_input = 0
    total_output = 0
    for _ in range(num_rounds):
        status, _, chat_body = srv.chat_completions("test-haiku")
        assert status == 200
        chat_usage = chat_body.get("usage", {})
        total_input += chat_usage.get("prompt_tokens", 0)
        total_output += chat_body.get("completion_tokens", 0)

    body = _fetch_usage(srv)
    limits = body["limits"]
    assert len(limits) > 0

    entry = limits[0]

    current = entry["current"]
    assert current["input_tokens"] >= total_input, \
        f"Current input_tokens {current['input_tokens']} should be >= total {total_input}"
    assert current["output_tokens"] >= total_output, \
        f"Current output_tokens {current['output_tokens']} should be >= total {total_output}"


def test_usage_api_limits_empty_when_no_limits(make_server):
    """When no limits are configured, the limits field is an empty list."""
    config_no_limits = """\
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
openai-compatibility:
  - name: "test-upstream"
    base-url: "{upstream_url}"
    api-key-entries:
      - api-key: "{upstream_key}"
    models:
      - name: "{upstream_model}"
        alias: "test-haiku"
        input_price_m: 3
        output_price_m: 15
        cache_price_m: 0.3
"""
    srv = make_server(config_no_limits)
    srv.start()

    status, _, _ = srv.chat_completions("test-haiku")
    assert status == 200

    body = _fetch_usage(srv)

    assert "limits" in body
    assert isinstance(body["limits"], list)
    assert len(body["limits"]) == 0, "limits should be empty when no limits are configured"


def test_usage_download_log_by_request_id(make_server):
    """The /v0/management/request-log-by-id/:id endpoint downloads log files by request ID."""
    srv = make_server(CONFIG_WITH_REQUEST_LOG)
    srv.start()

    status, _, _ = srv.chat_completions("test-haiku")
    assert status == 200

    body = _fetch_usage(srv)
    usage = body["usage"]

    # Extract request_id from a detail entry
    request_id = None
    for _ak, api_data in usage["apis"].items():
        for _mk, model_data in api_data["models"].items():
            for detail in model_data["details"]:
                if detail.get("request_id"):
                    request_id = detail["request_id"]
                    break
            if request_id:
                break
        if request_id:
            break

    assert request_id is not None and len(request_id) > 0, \
        "Should have a request_id in usage details"

    # Download the log file by request ID — should succeed
    log_path = f"/v0/management/request-log-by-id/{request_id}"
    status, _, resp_body = srv.request(log_path, headers=_mgmt_headers())
    assert status == 200, f"Log download returned {status}: {resp_body}"

    # Non-existent request ID should return 404
    status, _, _ = srv.request(
        "/v0/management/request-log-by-id/nonexistent123", headers=_mgmt_headers()
    )
    assert status == 404, f"Non-existent request ID should return 404, got {status}"
