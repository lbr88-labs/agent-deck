import asyncio
import json
from subprocess import CompletedProcess
from unittest.mock import AsyncMock, patch

import bridge


def output(data):
    return CompletedProcess([], 0, json.dumps(data), "")


def test_busy_enqueue_is_durable_and_idempotent():
    with patch("bridge.run_cli", return_value=output({"id": "incoming-event"})) as cli:
        assert bridge.send_to_conductor("session", "body", "work", request_id="incoming-event") == (
            True, "", "incoming-event")
    assert cli.call_args.args == ("message", "notify", "--producer", "operator", "--json",
                                  "--id", "incoming-event", "--", "session", "body")


def test_uncertain_enqueue_does_not_retry_or_wait():
    with patch("bridge.run_cli", return_value=output({})) as cli:
        assert bridge.send_to_conductor("session", "body", wait_for_reply=True) == (False, "", None)
        assert cli.call_count == 1


def test_wait_timeout_preserves_accepted_receipt():
    with patch("bridge.run_cli", return_value=output({"id": "a"})) as cli:
        assert bridge.send_to_conductor("session", "body", wait_for_reply=True, response_timeout=0) == (
            False, "", "a")
        assert cli.call_count == 1


def test_already_received_reply_uses_original_receipt():
    with patch("bridge.run_cli", side_effect=[output({"id": "a"}), output({"replies": [{"body":"done"}]})]) as cli:
        assert bridge.send_to_conductor("session", "body", wait_for_reply=True) == (True, "done", None)
        assert cli.call_args.args[:7] == ("message","wait","--producer","operator","--response-to","a","--reply-only")


def test_late_callback_forwards_only_original_reply():
    async def exercise():
        callback = AsyncMock()
        with patch("bridge.run_cli", return_value=output({"replies": [{"body":"only my result"}]})) as cli:
            bridge._register_pending_reply("mine", "work", callback)
            await asyncio.gather(*bridge._pending_reply_tasks)
        callback.assert_awaited_once_with("only my result")
        assert cli.call_args.args[:7] == ("message","wait","--producer","operator","--response-to","mine","--reply-only")
    asyncio.run(exercise())
