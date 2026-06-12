package session

// MaestroSessionTitle is the exact title of the fleet-supervisor session.
//
// Maestro is the orchestrator-of-orchestrators: it sits one level above
// conductors, exactly as conductors sit above child sessions (see
// conductor/agent-deck/maestro-design.md). Phase-1 identification is by
// exact title — the same convention conductors used before graduating to
// the is_conductor column in the v4 schema migration. When the is_maestro
// column ships (Phase 2), IsMaestro is the single seam to swap.
const MaestroSessionTitle = "conductor-maestro"

// IsMaestro reports whether this instance is the fleet supervisor.
// Exact match only: worker sessions titled "maestro-<something>" are
// regular sessions and must not be detected as the supervisor.
func (i *Instance) IsMaestro() bool {
	return i.Title == MaestroSessionTitle
}
