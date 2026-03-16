package reference

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed apple_role_capabilities.json
var raw []byte

var (
	load sync.Once
	snap *Snapshot
	err  error
)

type Snapshot struct {
	LastVerified string            `json:"last_verified"`
	Purpose      string            `json:"purpose"`
	Limitations  []string          `json:"limitations,omitempty"`
	Sources      []Source          `json:"sources"`
	APIKeyNotes  APIKeyNotes       `json:"api_key_notes"`
	Groups       []CapabilityGroup `json:"capability_groups"`
	Roles        []Role            `json:"roles"`
}

type Source struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type APIKeyNotes struct {
	Team       TeamKeyNotes       `json:"team_keys"`
	Individual IndividualKeyNotes `json:"individual_keys"`
}

type TeamKeyNotes struct {
	RequiredCreatorRoles []string `json:"required_creator_roles"`
	TeamScope            string   `json:"team_scope"`
	EditableAfterCreate  bool     `json:"editable_after_creation"`
	SelectableRoles      []string `json:"selectable_roles"`
}

type IndividualKeyNotes struct {
	EligibleUserRoles           []string `json:"eligible_user_roles"`
	OneActiveKeyPerUser         bool     `json:"one_active_key_per_user"`
	DefaultGenerationPermission string   `json:"default_generation_permission"`
	ManageableBy                []string `json:"manageable_by"`
}

type CapabilityGroup struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Summary string `json:"summary,omitempty"`
}

type Role struct {
	Code                  string   `json:"code"`
	Label                 string   `json:"label"`
	UIAliases             []string `json:"ui_aliases,omitempty"`
	TeamKeySelectable     bool     `json:"team_key_selectable"`
	IndividualKeyEligible bool     `json:"individual_key_eligible"`
	Summary               string   `json:"summary,omitempty"`
	Capabilities          []string `json:"capabilities,omitempty"`
	Notes                 []string `json:"notes,omitempty"`
	ExampleTasks          []string `json:"example_tasks,omitempty"`
	NotableActions        []string `json:"notable_exclusive_actions,omitempty"`
}

type View struct {
	LastVerified     string             `json:"lastVerified"`
	Purpose          string             `json:"purpose,omitempty"`
	Sources          []Source           `json:"sources,omitempty"`
	Scope            *Scope             `json:"scope,omitempty"`
	KeyNotes         *KeyNotes          `json:"keyNotes,omitempty"`
	RoleDetails      []Role             `json:"roleDetails,omitempty"`
	Capabilities     []CapabilityGroup  `json:"capabilities,omitempty"`
	DocumentedAccess []DocumentedAccess `json:"documentedAccess,omitempty"`
	UnknownRoles     []string           `json:"unknownRoles,omitempty"`
	Limitations      []string           `json:"limitations,omitempty"`
}

type Scope struct {
	AppliesToAllApps bool   `json:"appliesToAllApps,omitempty"`
	Summary          string `json:"summary,omitempty"`
}

type KeyNotes struct {
	Kind                        string   `json:"kind"`
	RequiredCreatorRoles        []string `json:"requiredCreatorRoles,omitempty"`
	SelectableRoles             []string `json:"selectableRoles,omitempty"`
	EditableAfterCreation       *bool    `json:"editableAfterCreation,omitempty"`
	EligibleUserRoles           []string `json:"eligibleUserRoles,omitempty"`
	OneActiveKeyPerUser         *bool    `json:"oneActiveKeyPerUser,omitempty"`
	DefaultGenerationPermission string   `json:"defaultGenerationPermission,omitempty"`
	ManageableBy                []string `json:"manageableBy,omitempty"`
}

type DocumentedAccess struct {
	ID         string   `json:"id"`
	Label      string   `json:"label"`
	Summary    string   `json:"summary,omitempty"`
	Roles      []string `json:"roles,omitempty"`
	RoleLabels []string `json:"roleLabels,omitempty"`
}

func Load() (*Snapshot, error) {
	load.Do(func() {
		var v Snapshot
		if e := json.Unmarshal(raw, &v); e != nil {
			err = e
			return
		}
		if e := validateSnapshot(&v); e != nil {
			err = e
			return
		}
		snap = &v
	})
	return snap, err
}

func validateSnapshot(v *Snapshot) error {
	if v == nil {
		return fmt.Errorf("reference snapshot is required")
	}

	groupByID := make(map[string]struct{}, len(v.Groups))
	for _, group := range v.Groups {
		id := strings.TrimSpace(group.ID)
		if id == "" {
			return fmt.Errorf("reference snapshot contains a capability group with an empty id")
		}
		groupByID[id] = struct{}{}
	}

	for _, role := range v.Roles {
		code := strings.TrimSpace(role.Code)
		if code == "" {
			return fmt.Errorf("reference snapshot contains a role with an empty code")
		}
		for _, rawID := range role.Capabilities {
			id := strings.TrimSpace(rawID)
			if id == "" {
				return fmt.Errorf("reference snapshot role %q contains an empty capability id", code)
			}
			if _, ok := groupByID[id]; !ok {
				return fmt.Errorf("reference snapshot role %q references unknown capability %q", code, id)
			}
		}
	}

	return nil
}

func Resolve(kind string, codes []string) (*View, error) {
	v, err := Load()
	if err != nil {
		return nil, err
	}

	roleByCode := make(map[string]Role, len(v.Roles))
	for _, role := range v.Roles {
		roleByCode[role.Code] = role
	}

	groupByID := make(map[string]CapabilityGroup, len(v.Groups))
	for _, group := range v.Groups {
		groupByID[group.ID] = group
	}

	seen := make(map[string]struct{}, len(codes))
	groups := make(map[string]struct{})
	view := &View{
		LastVerified: v.LastVerified,
		Purpose:      v.Purpose,
		Sources:      append([]Source(nil), v.Sources...),
		Limitations:  append([]string(nil), v.Limitations...),
	}
	switch strings.TrimSpace(kind) {
	case "team":
		editable := v.APIKeyNotes.Team.EditableAfterCreate
		view.Scope = &Scope{
			AppliesToAllApps: true,
			Summary:          v.APIKeyNotes.Team.TeamScope,
		}
		view.KeyNotes = &KeyNotes{
			Kind:                  "team",
			RequiredCreatorRoles:  append([]string(nil), v.APIKeyNotes.Team.RequiredCreatorRoles...),
			SelectableRoles:       append([]string(nil), v.APIKeyNotes.Team.SelectableRoles...),
			EditableAfterCreation: &editable,
		}
	case "individual":
		one := v.APIKeyNotes.Individual.OneActiveKeyPerUser
		view.KeyNotes = &KeyNotes{
			Kind:                        "individual",
			EligibleUserRoles:           append([]string(nil), v.APIKeyNotes.Individual.EligibleUserRoles...),
			OneActiveKeyPerUser:         &one,
			DefaultGenerationPermission: v.APIKeyNotes.Individual.DefaultGenerationPermission,
			ManageableBy:                append([]string(nil), v.APIKeyNotes.Individual.ManageableBy...),
		}
	}

	groupRoles := make(map[string][]string)
	groupRoleLabels := make(map[string][]string)

	for _, rawCode := range codes {
		code := strings.TrimSpace(rawCode)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}

		role, ok := roleByCode[code]
		if !ok {
			view.UnknownRoles = append(view.UnknownRoles, code)
			continue
		}
		view.RoleDetails = append(view.RoleDetails, role)
		for _, id := range role.Capabilities {
			groups[id] = struct{}{}
			groupRoles[id] = append(groupRoles[id], role.Code)
			groupRoleLabels[id] = append(groupRoleLabels[id], role.Label)
		}
	}

	for _, group := range v.Groups {
		if _, ok := groups[group.ID]; ok {
			view.Capabilities = append(view.Capabilities, group)
			view.DocumentedAccess = append(view.DocumentedAccess, DocumentedAccess{
				ID:         group.ID,
				Label:      group.Label,
				Summary:    group.Summary,
				Roles:      append([]string(nil), groupRoles[group.ID]...),
				RoleLabels: append([]string(nil), groupRoleLabels[group.ID]...),
			})
		}
	}

	return view, nil
}
