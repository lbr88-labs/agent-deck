# lifecycle-fork.tape — the fork precondition guard, recorded against the real binary.
#
# A full fork happy-path (inheriting a live Claude conversation) needs a real
# ClaudeSessionID from a key-gated, non-deterministic claude transcript, so it
# is a documented Tier N gap (see docs/testing/capability-video-gaps.md and
# tests/capability/lifecycle_test.go). What we CAN show authentically is the
# real guard: forking a non-Claude (bash/echobot) session is refused with a
# clear error and creates no child row. Asserting twin:
# TestCapability_Lifecycle_Fork.

Output "docs/status/videos/lifecycle-fork.webm"
Output "docs/status/videos/lifecycle-fork.gif"

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

Type "# agent-deck capability: fork only works on a Claude session (precondition guard)"
Enter
Sleep 1s
Type "agent-deck add -c bash -t cap-fork $CAPVID_PROJECT"
Enter
Sleep 2s

Type "# try to fork a non-Claude session: the guard refuses it"
Enter
Sleep 700ms
Type "agent-deck session fork cap-fork"
Enter
Sleep 2500ms

Type "# and no child row was created"
Enter
Sleep 700ms
Type "agent-deck list"
Enter
Sleep 3s
