package cmd

import "testing"

func TestRootCommandForArgsBuildsOnlySelectedSubtree(t *testing.T) {
	root := rootCommandForArgs("test", []string{"builds", "list"})
	builds := findDirectSubcommand(root, "builds")
	if builds == nil || len(builds.Subcommands) == 0 {
		t.Fatal("selected builds subtree was not materialized")
	}
	apps := findDirectSubcommand(root, "apps")
	if apps == nil {
		t.Fatal("apps metadata stub is missing")
	}
	if len(apps.Subcommands) != 0 {
		t.Fatal("unselected apps subtree was materialized")
	}
}

func TestRootCommandForArgsSkipsRootFlagValues(t *testing.T) {
	root := rootCommandForArgs("test", []string{"--profile", "builds", "completion", "--shell", "bash"})
	completion := findDirectSubcommand(root, "completion")
	if completion == nil || completion.FlagSet == nil {
		t.Fatal("completion command was not materialized")
	}
	builds := findDirectSubcommand(root, "builds")
	if builds == nil || len(builds.Subcommands) != 0 {
		t.Fatal("root flag value was mistaken for the selected command")
	}
}

func TestRootCommandForArgsPreservesRootHelp(t *testing.T) {
	full := RootCommand("test")
	lazy := rootCommandForArgs("test", []string{"--help"})
	if got, want := lazy.UsageFunc(lazy), full.UsageFunc(full); got != want {
		t.Fatalf("lazy root help differs from full catalog:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func BenchmarkRootCommandForArgsBuildsList(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = rootCommandForArgs("test", []string{"builds", "list"})
	}
}

func BenchmarkRootCommandForArgsHelp(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = rootCommandForArgs("test", []string{"--help"})
	}
}
