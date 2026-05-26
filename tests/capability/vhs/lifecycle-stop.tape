# lifecycle-stop.tape — start a real session, then stop it, watching the state flip.
#
# Records the real binary: launch a session (real tmux pane on the isolated
# socket), show it running in `list`, run the real `session stop`, then show the
# row return to stopped. Asserting twin: TestCapability_Lifecycle_Stop in
# tests/capability/lifecycle_test.go.

Output "docs/status/videos/lifecycle-stop.webm"
Output "docs/status/videos/lifecycle-stop.gif"

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

Type "# agent-deck capability: stop a running session, watch the state change"
Enter
Sleep 1s
Type "agent-deck launch $CAPVID_PROJECT -c echobot -t cap-stop"
Enter
Sleep 2500ms

Type "# running now (status reported by the registry)"
Enter
Sleep 600ms
Type `agent-deck list --json | grep -E '"(title|status)"'`
Enter
Sleep 2500ms

Type "# stop it: the real tmux pane is torn down"
Enter
Sleep 600ms
Type "agent-deck session stop cap-stop"
Enter
Sleep 2s

Type "# the registry returns the session to stopped"
Enter
Sleep 600ms
Type `agent-deck list --json | grep -E '"(title|status)"'`
Enter
Sleep 3s
