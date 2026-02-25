package cmd

import (
	"flag"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type rootCommandGroup struct {
	title    string
	commands []string
}

var rootUsageGroups = []rootCommandGroup{
	{
		title:    "GETTING STARTED COMMANDS",
		commands: []string{"auth", "doctor", "install-skills", "init", "docs"},
	},
	{
		title:    "EXPERIMENTAL COMMANDS",
		commands: []string{"web"},
	},
	{
		title:    "ANALYTICS & FINANCE COMMANDS",
		commands: []string{"analytics", "insights", "finance", "performance", "feedback", "crashes"},
	},
	{
		title: "APP MANAGEMENT COMMANDS",
		commands: []string{
			"apps", "app-setup", "app-tags", "app-info", "app-infos", "versions",
			"localizations", "screenshots", "video-previews", "background-assets", "product-pages",
			"routing-coverage", "pricing", "pre-orders", "categories", "age-rating",
			"accessibility", "encryption", "eula", "agreements", "app-clips",
			"android-ios-mapping", "marketplace", "alternative-distribution",
			"nominations", "game-center",
		},
	},
	{
		title: "TESTFLIGHT & BUILD COMMANDS",
		commands: []string{
			"testflight", "builds", "build-bundles", "pre-release-versions",
			"build-localizations", "beta-app-localizations", "beta-build-localizations",
			"sandbox",
		},
	},
	{
		title:    "REVIEW & RELEASE COMMANDS",
		commands: []string{"review", "reviews", "submit", "validate", "publish"},
	},
	{
		title:    "MONETIZATION COMMANDS",
		commands: []string{"iap", "app-events", "subscriptions", "offer-codes", "win-back-offers", "promoted-purchases"},
	},
	{
		title:    "SIGNING COMMANDS",
		commands: []string{"signing", "bundle-ids", "certificates", "profiles", "merchant-ids", "pass-type-ids", "notarization"},
	},
	{
		title:    "TEAM & ACCESS COMMANDS",
		commands: []string{"account", "users", "actors", "devices"},
	},
	{
		title:    "AUTOMATION COMMANDS",
		commands: []string{"webhooks", "xcode-cloud", "notify", "migrate"},
	},
	{
		title:    "UTILITY COMMANDS",
		commands: []string{"version", "completion"},
	},
}

// RootUsageFunc renders grouped root help, similar to gh style.
func RootUsageFunc(c *ffcli.Command) string {
	var b strings.Builder

	shortHelp := strings.TrimSpace(c.ShortHelp)
	longHelp := strings.TrimSpace(c.LongHelp)
	if shortHelp == "" && longHelp != "" {
		shortHelp = longHelp
		longHelp = ""
	}

	if shortHelp != "" {
		b.WriteString(shared.Bold("DESCRIPTION"))
		b.WriteString("\n")
		b.WriteString("  ")
		b.WriteString(shortHelp)
		b.WriteString("\n\n")
	}

	usage := strings.TrimSpace(c.ShortUsage)
	if usage == "" {
		usage = strings.TrimSpace(c.Name)
	}
	if usage != "" {
		b.WriteString(shared.Bold("USAGE"))
		b.WriteString("\n")
		b.WriteString("  ")
		b.WriteString(usage)
		b.WriteString("\n\n")
	}

	if longHelp != "" {
		if shortHelp != "" && strings.HasPrefix(longHelp, shortHelp) {
			longHelp = strings.TrimSpace(strings.TrimPrefix(longHelp, shortHelp))
		}
		if longHelp != "" {
			b.WriteString(longHelp)
			b.WriteString("\n\n")
		}
	}

	writeRootGroupedSubcommands(&b, c.Subcommands)
	writeRootFlags(&b, c.FlagSet)
	return b.String()
}

func writeRootGroupedSubcommands(b *strings.Builder, subcommands []*ffcli.Command) {
	if len(subcommands) == 0 {
		return
	}

	byName := make(map[string]*ffcli.Command, len(subcommands))
	for _, sub := range subcommands {
		byName[sub.Name] = sub
	}

	rendered := make(map[string]bool, len(subcommands))
	for _, group := range rootUsageGroups {
		groupCommands := make([]*ffcli.Command, 0, len(group.commands))
		for _, name := range group.commands {
			sub, ok := byName[name]
			if !ok || rendered[name] {
				continue
			}
			rendered[name] = true
			groupCommands = append(groupCommands, sub)
		}
		if len(groupCommands) == 0 {
			continue
		}

		b.WriteString(shared.Bold(group.title))
		b.WriteString("\n")
		tw := tabwriter.NewWriter(b, 0, 2, 2, ' ', 0)
		for _, sub := range groupCommands {
			_, _ = fmt.Fprintf(tw, "  %s:\t%s\n", sub.Name, sub.ShortHelp)
		}
		_ = tw.Flush()
		b.WriteString("\n")
	}

	additional := make([]*ffcli.Command, 0)
	for _, sub := range subcommands {
		if !rendered[sub.Name] {
			additional = append(additional, sub)
		}
	}
	if len(additional) == 0 {
		return
	}

	b.WriteString(shared.Bold("ADDITIONAL COMMANDS"))
	b.WriteString("\n")
	tw := tabwriter.NewWriter(b, 0, 2, 2, ' ', 0)
	for _, sub := range additional {
		_, _ = fmt.Fprintf(tw, "  %s:\t%s\n", sub.Name, sub.ShortHelp)
	}
	_ = tw.Flush()
	b.WriteString("\n")
}

func writeRootFlags(b *strings.Builder, fs *flag.FlagSet) {
	if fs == nil {
		return
	}

	hasFlags := false
	fs.VisitAll(func(*flag.Flag) {
		hasFlags = true
	})
	if !hasFlags {
		return
	}

	b.WriteString(shared.Bold("FLAGS"))
	b.WriteString("\n")
	tw := tabwriter.NewWriter(b, 0, 2, 2, ' ', 0)
	fs.VisitAll(func(f *flag.Flag) {
		usage := f.Usage
		if f.Name == "output" {
			usage = strings.Replace(usage, "json (default),", "json,", 1)
		}
		if f.DefValue != "" {
			_, _ = fmt.Fprintf(tw, "  --%s\t%s (default: %s)\n", f.Name, usage, f.DefValue)
			return
		}
		_, _ = fmt.Fprintf(tw, "  --%s\t%s\n", f.Name, usage)
	})
	_ = tw.Flush()
	b.WriteString("\n")
}
