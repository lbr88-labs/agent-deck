package session

// RefreshInstancesForCLIStatus is the CLI analogue of
// SessionDataService.refreshStatuses (internal/web) and
// Home.backgroundStatusUpdate (internal/ui). CLI JSON emitters must call
// it before iterating inst.UpdateStatus() so the tmux pane-title cache is
// warm and on-disk hook statuses are loaded — without this step the
// title fast-path in tmux.GetStatus cannot fire (issue #610).
//
// TDD stub: the real implementation is pending as part of fix/issue-610.
// Until then this is a no-op, which keeps go vet/build green while the
// regression tests in instance_cli_parity_test.go stay red.
func RefreshInstancesForCLIStatus(instances []*Instance) {
	_ = instances
}
