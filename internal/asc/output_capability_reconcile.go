package asc

// CapabilityReconcilePlan is the result of entitlements-to-capability planning.
type CapabilityReconcilePlan struct {
	BundleID string                      `json:"bundleId"`
	Actions  []CapabilityReconcileAction `json:"actions"`
}

// CapabilityReconcileAction is one planned capability change.
type CapabilityReconcileAction struct {
	Action       string              `json:"action"`
	Capability   string              `json:"capability,omitempty"`
	Entitlement  string              `json:"entitlement,omitempty"`
	CapabilityID string              `json:"capabilityId,omitempty"`
	Command      string              `json:"command,omitempty"`
	Settings     []CapabilitySetting `json:"settings,omitempty"`
}

func capabilityReconcilePlanRows(plan *CapabilityReconcilePlan) ([]string, [][]string) {
	headers := []string{"Action", "Capability", "Entitlement", "Capability ID"}
	if plan == nil {
		return headers, nil
	}
	rows := make([][]string, 0, len(plan.Actions))
	for _, action := range plan.Actions {
		rows = append(rows, []string{action.Action, action.Capability, action.Entitlement, action.CapabilityID})
	}
	return headers, rows
}
