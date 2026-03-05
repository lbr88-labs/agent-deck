import { test, describe } from "repterm";

// Tests remote session creation via the SSH runner (the core logic that n/N keys invoke).
// This validates that CreateSession correctly runs "add --quick --json" + "session start"
// on the remote, and that the session is created, started, and attachable.

describe("Remote Session Creation", () => {
  test("add + start creates a running session on remote", async ({ terminal }) => {
    // Count sessions before
    const before = await terminal.run(
      `/usr/bin/ssh manjaro "/home/xiangpan/agent-deck list --json 2>/dev/null || echo '[]'"`,
      { silent: true }
    );
    let initialCount = 0;
    try {
      const s = JSON.parse(before.stdout);
      initialCount = Array.isArray(s) ? s.length : 0;
    } catch {}

    // Step 1: Create session (same as CreateSession step 1)
    const create = await terminal.run(
      `/usr/bin/ssh manjaro "/home/xiangpan/agent-deck add --quick --json"`,
    );
    if (create.code !== 0) {
      throw new Error(`add failed (exit ${create.code}): ${create.stderr || create.stdout}`);
    }

    let sessionId = "";
    let sessionTitle = "";
    try {
      const result = JSON.parse(create.stdout);
      sessionId = result.id;
      sessionTitle = result.title;
    } catch (e) {
      throw new Error(`Failed to parse add output: ${create.stdout}`);
    }
    if (!sessionId) throw new Error("add returned empty session ID");

    // Step 2: Start session (same as CreateSession step 2)
    const start = await terminal.run(
      `/usr/bin/ssh manjaro "/home/xiangpan/agent-deck session start '${sessionTitle}'"`,
      { timeout: 20000 }
    );
    if (start.code !== 0) {
      throw new Error(`session start failed (exit ${start.code}): ${start.stderr || start.stdout}`);
    }

    // Verify session exists and is started
    const after = await terminal.run(
      `/usr/bin/ssh manjaro "/home/xiangpan/agent-deck list --json"`,
      { silent: true }
    );
    let finalCount = 0;
    let newSession: any = null;
    try {
      const s = JSON.parse(after.stdout);
      if (Array.isArray(s)) {
        finalCount = s.length;
        newSession = s.find((x: any) => x.id === sessionId);
      }
    } catch {}

    if (finalCount <= initialCount) {
      throw new Error(`Expected more sessions. Before: ${initialCount}, After: ${finalCount}`);
    }
    if (!newSession) {
      throw new Error(`Session ${sessionId} not found in list`);
    }

    // Session should not be in error state after start
    if (newSession.status === "error") {
      throw new Error(`Session ${sessionId} is in error state after start`);
    }

    console.log(`  Created+started: ${newSession.title} (${sessionId}) status=${newSession.status}`);
  });

  test("session is attachable after create+start", async ({ terminal }) => {
    // Get the latest session
    const list = await terminal.run(
      `/usr/bin/ssh manjaro "/home/xiangpan/agent-deck list --json"`,
      { silent: true }
    );
    const sessions = JSON.parse(list.stdout);
    const latest = sessions[sessions.length - 1];

    // Verify attach doesn't fail with "not found"
    const attach = await terminal.run(
      `/usr/bin/ssh -t manjaro "/home/xiangpan/agent-deck session attach ${latest.id}" < /dev/null`,
      { timeout: 5000 }
    );
    if (attach.stderr?.includes("not found") || attach.stdout?.includes("not found")) {
      throw new Error(`Session ${latest.id} not found for attach`);
    }

    console.log(`  Attach check for ${latest.title} (${latest.id}): OK`);
  });
});
