package asc

import "testing"

func TestCapabilityReconcileTableShowsWebCommand(t *testing.T) {
	plan := &CapabilityReconcilePlan{Actions: []CapabilityReconcileAction{{Action: "needsWebSession", Command: "asc web bundle-ids capabilities enable --capability INCREASED_MEMORY_LIMIT"}}}
	headers, rows := capabilityReconcilePlanRows(plan)
	if len(headers) != len(rows[0]) || headers[len(headers)-1] != "Command" || rows[0][len(rows[0])-1] != plan.Actions[0].Command {
		t.Fatalf("headers=%q rows=%q", headers, rows)
	}
}
