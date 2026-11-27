package registry

type commandBehavior struct {
	WriteIntent      string   `json:"write_intent"`
	WriteCondition   string   `json:"write_condition,omitempty"`
	HumanRequirement string   `json:"human_requirement"`
	HumanSteps       []string `json:"human_steps,omitempty"`
}

func behaviorForCommand(path string) commandBehavior {
	switch path {
	case "commands", "search", "capabilities", "schema", "version",
		"apps list", "apps view", "app-clips review-details view",
		"review details plan", "review details-for-version", "review details-get",
		"catalog plan", "metadata validate":
		return commandBehavior{WriteIntent: "none", HumanRequirement: "none"}
	case "metadata pull":
		return commandBehavior{WriteIntent: "local", HumanRequirement: "none"}
	case "review details set", "review details-update", "review details-create",
		"app-clips review-details create", "app-clips review-details update":
		return commandBehavior{WriteIntent: "remote", HumanRequirement: "none"}
	case "catalog apply":
		return commandBehavior{
			WriteIntent: "remote", HumanRequirement: "conditional",
			HumanSteps: []string{"Approve removal of existing availability or replacement of an offer before passing --confirm."},
		}
	case "metadata push":
		return commandBehavior{
			WriteIntent: "conditional", WriteCondition: "--dry-run previews changes without writing to App Store Connect.",
			HumanRequirement: "conditional", HumanSteps: []string{"Approve deletes, field clearing, review-directory application, or App Clip and preview asset writes before passing --confirm."},
		}
	case "review submit":
		return commandBehavior{
			WriteIntent: "conditional", WriteCondition: "--dry-run previews submission without writing to App Store Connect.",
			HumanRequirement: "conditional", HumanSteps: []string{"Approve submission before passing --confirm."},
		}
	case "auth doctor", "doctor":
		return commandBehavior{
			WriteIntent: "conditional", WriteCondition: "--fix can repair local credential configuration.",
			HumanRequirement: "conditional", HumanSteps: []string{"Approve local repairs before passing --fix --confirm."},
		}
	case "api":
		return commandBehavior{
			WriteIntent: "conditional", WriteCondition: "GET reads App Store Connect. POST, PATCH, and DELETE can write.",
			HumanRequirement: "conditional", HumanSteps: []string{"Approve the exact request before passing --confirm for a write."},
		}
	case "web auth login", "web auth capabilities":
		return commandBehavior{
			WriteIntent: "conditional", WriteCondition: "Sign-in can save local credentials or a session; explicit provider selection can change the session's active team.",
			HumanRequirement: "conditional",
			HumanSteps:       []string{"Provide the Apple Account password if prompted.", "Complete Apple's verification challenge if needed, including entering a code or touching a security key."},
		}
	case "web apps create":
		return commandBehavior{
			WriteIntent: "remote", HumanRequirement: "conditional",
			HumanSteps: []string{"Provide missing app creation fields if prompted.", "Complete Apple's sign-in and verification if a cached session is unavailable."},
		}
	default:
		return commandBehavior{WriteIntent: "unknown", HumanRequirement: "unknown"}
	}
}
