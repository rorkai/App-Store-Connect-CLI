package asc

// CapabilityReconcilePlan is the result of entitlements-to-capability planning.
type CapabilityReconcilePlan struct {
	BundleID        string                      `json:"bundleId"`
	Actions         []CapabilityReconcileAction `json:"actions"`
	IdentifierLinks []CapabilityIdentifierLink  `json:"identifierLinks,omitempty"`
}

// CapabilityReconcileAction is one planned capability change.
type CapabilityReconcileAction struct {
	Action       string              `json:"action"`
	Capability   string              `json:"capability,omitempty"`
	Entitlement  string              `json:"entitlement,omitempty"`
	CapabilityID string              `json:"capabilityId,omitempty"`
	Command      string              `json:"command,omitempty"`
	Settings     []CapabilitySetting `json:"settings,omitempty"`
	Status       string              `json:"status,omitempty"`
	Error        string              `json:"error,omitempty"`
}

func capabilityReconcilePlanRows(plan *CapabilityReconcilePlan) ([]string, [][]string) {
	headers := []string{"Action", "Capability", "Entitlement", "Capability ID", "Status", "Error", "Command"}
	if plan == nil {
		return headers, nil
	}
	rows := make([][]string, 0, len(plan.Actions))
	for _, action := range plan.Actions {
		rows = append(rows, []string{action.Action, action.Capability, action.Entitlement, action.CapabilityID, action.Status, action.Error, action.Command})
	}
	return headers, rows
}

// CapabilityIdentifierLink reports evidence independently of capability state.
type CapabilityIdentifierLink struct {
	Entitlement string `json:"entitlement"`
	Identifier  string `json:"identifier"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
	Command     string `json:"command"`
}

func capabilityReconcilePlanTables(plan *CapabilityReconcilePlan, render func([]string, [][]string)) error {
	headers, rows := capabilityReconcilePlanRows(plan)
	render(headers, rows)
	if plan == nil || len(plan.IdentifierLinks) == 0 {
		return nil
	}
	rows = nil
	for _, link := range plan.IdentifierLinks {
		rows = append(rows, []string{link.Entitlement, link.Identifier, link.Status, link.Reason, link.Command})
	}
	render([]string{"Entitlement", "Identifier", "Link Status", "Reason", "Command"}, rows)
	return nil
}
