import json
from subprocess import CompletedProcess
from unittest.mock import patch

from bridge import send_heartbeat


def result(data):
    return CompletedProcess([], 0, json.dumps(data), "")


def test_heartbeat_waits_for_a_correlated_reply_without_resending():
    responses = [
        result({"id": "request-1"}),
        result({"timed_out": True, "replies": []}),
        result({"replies": [{"body": "NEED: check worker status"}]}),
    ]
    with patch("bridge.run_cli", side_effect=responses) as cli:
        assert send_heartbeat("conductor-main", "report status", "work") == (
            True, "NEED: check worker status"
        )
    sent, first, second = cli.call_args_list
    assert sent.args[:4] == ("message", "notify", "--producer", "heartbeat")
    for wait in (first, second):
        assert wait.args[:7] == (
            "message", "wait", "--producer", "heartbeat",
            "--response-to", "request-1", "--reply-only",
        )
