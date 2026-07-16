package subscriptions

import "testing"

func TestSubscriptionsVersionsCommand_Surface(t *testing.T) {
	cmd := SubscriptionsVersionsCommand()
	want := map[string]bool{
		"create":        false,
		"list":          false,
		"view":          false,
		"links":         false,
		"localizations": false,
		"images":        false,
	}
	for _, subcommand := range cmd.Subcommands {
		if _, ok := want[subcommand.Name]; ok {
			want[subcommand.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("missing subscriptions versions %s command", name)
		}
	}
}
