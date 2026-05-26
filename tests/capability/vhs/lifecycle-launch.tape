# lifecycle-launch.tape — the atomic launch path, recorded against the real binary.
#
# `agent-deck launch` is add + start + (optional) send in one command. This clip
# runs it for real with a message, then shows the registry row appear and the
# echobot's genuine reply in the pane. Asserting twin:
# TestCapability_Lifecycle_Launch in tests/capability/roundtrip_test.go.

Output "docs/status/videos/lifecycle-launch.webm"
Output "docs/status/videos/lifecycle-launch.gif"

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

Type "# agent-deck capability: launch (add + start + send in one command)"
Enter
Sleep 1s
Type "agent-deck launch $CAPVID_PROJECT -c echobot -t cap-launch -m PINGLAUNCH-demo"
Enter
Sleep 3500ms

Type "# the session is registered and running"
Enter
Sleep 700ms
Type "agent-deck list"
Enter
Sleep 2500ms

Type "# and the agent already echoed the launch message"
Enter
Sleep 700ms
Type `agent-deck session output cap-launch --pane | grep -E 'READY > PINGLAUNCH|ECHO:PINGLAUNCH'`
Enter
Sleep 3s

Hide
Type "agent-deck session stop cap-launch"
Enter
Sleep 1s
Show
