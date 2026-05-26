# echo-roundtrip.tape — the headline capability clip.
#
# Watch agent-deck talk to a live agent: launch a real echobot session in an
# isolated sandbox, send a visible token, and see the agent echo it back. This
# drives the exact production path a real `claude` send takes (readiness gate ->
# send-keys + Enter -> capture-pane read-back); only the brain on the far end is
# deterministic. See tests/capability/roundtrip_test.go for the asserting twin.
#
# Rendered by scripts/capability-videos.sh, which exports CAPVID_REPO / CAPVID_BIN
# / CAPVID_PROJECT and builds the binary once. The Hide block sources the shared
# sandbox (isolated HOME + per-recording tmux socket, TMUX unset) off camera.

Output "docs/status/videos/echo-roundtrip.webm"
Output "docs/status/videos/echo-roundtrip.gif"

Set Shell "bash"
Set FontSize 18
Set Width 1200
Set Height 640
Set Padding 24
Set TypingSpeed 45ms

Hide
Type "source $CAPVID_REPO/tests/capability/vhs/lib/sandbox.sh"
Enter
Sleep 2s
Show

Type "# agent-deck capability: send a message to a live agent, get a reply back"
Enter
Sleep 1s
Type "# 1) launch a real echobot session (add + start a tmux pane, isolated)"
Enter
Sleep 800ms
Type "agent-deck launch $CAPVID_PROJECT -c echobot -t cap-echo"
Enter
Sleep 2500ms

Type "# 2) send a visible token to the agent"
Enter
Sleep 800ms
Type "agent-deck session send cap-echo PING-DEMO-7f3a --no-wait"
Enter
Sleep 3500ms

Type "# 3) read the pane back: the agent echoed our token"
Enter
Sleep 800ms
Type `agent-deck session output cap-echo --pane | grep -E 'READY > PING-DEMO|ECHO:PING-DEMO'`
Enter
Sleep 3s

Type "# 4) tear it down (stop the pane, drop the row)"
Enter
Sleep 600ms
Type "agent-deck session stop cap-echo && agent-deck session remove cap-echo"
Enter
Sleep 2s
