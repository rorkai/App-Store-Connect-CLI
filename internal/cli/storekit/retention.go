package storekit

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	storekitapi "github.com/rudrankriyam/App-Store-Connect-CLI/internal/storekit"
)

func RetentionMessagingCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "retention-messaging",
		ShortUsage: "asc storekit retention-messaging <subcommand> [flags]",
		ShortHelp:  "Manage subscription Retention Messaging resources.",
		LongHelp: `Manage Apple's subscription Retention Messaging resources.

Access requires Apple approval and a dedicated In-App Purchase API key.

Examples:
  asc storekit retention-messaging images list --environment sandbox
  asc storekit retention-messaging messages upload --message-id UUID --file message.json --environment sandbox
  asc storekit retention-messaging defaults set --product-id com.example.monthly --locale en-US --message-id UUID --environment production
  asc storekit retention-messaging endpoint set --url https://example.com/retention --environment production`,
		FlagSet: fs,
		Subcommands: []*ffcli.Command{
			imagesCommand(),
			messagesCommand(),
			defaultsCommand(),
			endpointCommand(),
			performanceCommand(),
		},
		UsageFunc: shared.DefaultUsageFunc,
		Exec:      func(ctx context.Context, args []string) error { return flag.ErrHelp },
	}
}

func imagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging images", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc storekit retention-messaging images <subcommand> [flags]",
		ShortHelp:  "Upload, list, and delete retention images.",
		FlagSet:    fs,
		Subcommands: []*ffcli.Command{
			imagesListCommand(), imagesUploadCommand(), imagesDeleteCommand(),
		},
		UsageFunc: shared.DefaultUsageFunc,
		Exec:      func(ctx context.Context, args []string) error { return flag.ErrHelp },
	}
}

func imagesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging images list", flag.ExitOnError)
	common := bindCommonFlags(fs)
	output := shared.BindOutputFlags(fs)
	return leafCommand("list", "asc storekit retention-messaging images list --environment ENV [flags]", "List retention images.", fs, func(ctx context.Context, args []string) error {
		if err := rejectUnexpectedArgs(args); err != nil {
			return err
		}
		client, _, err := resolveClient(ctx, common)
		if err != nil {
			return usageOrWrap("storekit retention-messaging images list", err)
		}
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		result, err := client.ListImages(requestCtx)
		if err != nil {
			return fmt.Errorf("storekit retention-messaging images list: %w", err)
		}
		rows := make([][]string, 0, len(result.ImageIdentifiers))
		for _, image := range result.ImageIdentifiers {
			rows = append(rows, []string{image.ImageIdentifier, string(image.ImageSize), image.ImageState})
		}
		return printOutput(result, *output.Output, *output.Pretty, []string{"Image ID", "Size", "State"}, rows)
	})
}

func imagesUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging images upload", flag.ExitOnError)
	imageID := fs.String("image-id", "", "Client-generated image UUID")
	file := fs.String("file", "", "Path to a PNG image")
	size := fs.String("image-size", string(storekitapi.ImageSizeFull), "Image size: FULL_SIZE or BULLET_POINT")
	common := bindCommonFlags(fs)
	output := shared.BindOutputFlags(fs)
	return leafCommand("upload", "asc storekit retention-messaging images upload --image-id UUID --file IMAGE.png --environment ENV [flags]", "Upload a PNG retention image.", fs, func(ctx context.Context, args []string) error {
		if err := rejectUnexpectedArgs(args); err != nil {
			return err
		}
		if err := requireFlag("--image-id", *imageID); err != nil {
			return err
		}
		if err := validateUUIDFlag("--image-id", *imageID); err != nil {
			return err
		}
		if err := requireFlag("--file", *file); err != nil {
			return err
		}
		imageSize := storekitapi.ImageSize(strings.ToUpper(strings.TrimSpace(*size)))
		if imageSize != storekitapi.ImageSizeFull && imageSize != storekitapi.ImageSizeBulletPoint {
			return shared.UsageError("--image-size must be one of: FULL_SIZE, BULLET_POINT")
		}
		data, err := readPNG(*file, imageSize)
		if err != nil {
			return shared.UsageError(err.Error())
		}
		client, environment, err := resolveClient(ctx, common)
		if err != nil {
			return usageOrWrap("storekit retention-messaging images upload", err)
		}
		requestCtx, cancel := shared.ContextWithUploadTimeout(ctx)
		defer cancel()
		if err := client.UploadImage(requestCtx, *imageID, imageSize, data); err != nil {
			return fmt.Errorf("storekit retention-messaging images upload: %w", err)
		}
		result := mutation("image", *imageID, "uploaded", environment)
		return printMutation(result, *output.Output, *output.Pretty)
	})
}

func imagesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging images delete", flag.ExitOnError)
	imageID := fs.String("image-id", "", "Image UUID")
	confirm := fs.Bool("confirm", false, "Confirm image deletion")
	common := bindCommonFlags(fs)
	output := shared.BindOutputFlags(fs)
	return leafCommand("delete", "asc storekit retention-messaging images delete --image-id UUID --environment ENV --confirm [flags]", "Delete a retention image.", fs, func(ctx context.Context, args []string) error {
		if err := rejectUnexpectedArgs(args); err != nil {
			return err
		}
		if !*confirm {
			return shared.UsageError("--confirm is required")
		}
		if err := requireFlag("--image-id", *imageID); err != nil {
			return err
		}
		if err := validateUUIDFlag("--image-id", *imageID); err != nil {
			return err
		}
		client, environment, err := resolveClient(ctx, common)
		if err != nil {
			return usageOrWrap("storekit retention-messaging images delete", err)
		}
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		if err := client.DeleteImage(requestCtx, *imageID); err != nil {
			return fmt.Errorf("storekit retention-messaging images delete: %w", err)
		}
		return printMutation(mutation("image", *imageID, "deleted", environment), *output.Output, *output.Pretty)
	})
}

func messagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging messages", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "messages",
		ShortUsage: "asc storekit retention-messaging messages <subcommand> [flags]",
		ShortHelp:  "Upload, list, and delete retention messages.",
		FlagSet:    fs,
		Subcommands: []*ffcli.Command{
			messagesListCommand(), messagesUploadCommand(), messagesDeleteCommand(),
		},
		UsageFunc: shared.DefaultUsageFunc,
		Exec:      func(ctx context.Context, args []string) error { return flag.ErrHelp },
	}
}

func messagesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging messages list", flag.ExitOnError)
	common := bindCommonFlags(fs)
	output := shared.BindOutputFlags(fs)
	return leafCommand("list", "asc storekit retention-messaging messages list --environment ENV [flags]", "List retention messages.", fs, func(ctx context.Context, args []string) error {
		if err := rejectUnexpectedArgs(args); err != nil {
			return err
		}
		client, _, err := resolveClient(ctx, common)
		if err != nil {
			return usageOrWrap("storekit retention-messaging messages list", err)
		}
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		result, err := client.ListMessages(requestCtx)
		if err != nil {
			return fmt.Errorf("storekit retention-messaging messages list: %w", err)
		}
		rows := make([][]string, 0, len(result.MessageIdentifiers))
		for _, message := range result.MessageIdentifiers {
			rows = append(rows, []string{message.MessageIdentifier, message.MessageState})
		}
		return printOutput(result, *output.Output, *output.Pretty, []string{"Message ID", "State"}, rows)
	})
}

func messagesUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging messages upload", flag.ExitOnError)
	messageID := fs.String("message-id", "", "Client-generated message UUID")
	file := fs.String("file", "", "Path to a retention message JSON file")
	common := bindCommonFlags(fs)
	output := shared.BindOutputFlags(fs)
	return leafCommand("upload", "asc storekit retention-messaging messages upload --message-id UUID --file MESSAGE.json --environment ENV [flags]", "Upload a retention message.", fs, func(ctx context.Context, args []string) error {
		if err := rejectUnexpectedArgs(args); err != nil {
			return err
		}
		if err := requireFlag("--message-id", *messageID); err != nil {
			return err
		}
		if err := validateUUIDFlag("--message-id", *messageID); err != nil {
			return err
		}
		if err := requireFlag("--file", *file); err != nil {
			return err
		}
		message, err := readMessage(*file)
		if err != nil {
			return shared.UsageError(err.Error())
		}
		client, environment, err := resolveClient(ctx, common)
		if err != nil {
			return usageOrWrap("storekit retention-messaging messages upload", err)
		}
		requestCtx, cancel := shared.ContextWithUploadTimeout(ctx)
		defer cancel()
		if err := client.UploadMessage(requestCtx, *messageID, message); err != nil {
			return fmt.Errorf("storekit retention-messaging messages upload: %w", err)
		}
		return printMutation(mutation("message", *messageID, "uploaded", environment), *output.Output, *output.Pretty)
	})
}

func messagesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging messages delete", flag.ExitOnError)
	messageID := fs.String("message-id", "", "Message UUID")
	confirm := fs.Bool("confirm", false, "Confirm message deletion")
	common := bindCommonFlags(fs)
	output := shared.BindOutputFlags(fs)
	return leafCommand("delete", "asc storekit retention-messaging messages delete --message-id UUID --environment ENV --confirm [flags]", "Delete a retention message.", fs, func(ctx context.Context, args []string) error {
		if err := rejectUnexpectedArgs(args); err != nil {
			return err
		}
		if !*confirm {
			return shared.UsageError("--confirm is required")
		}
		if err := requireFlag("--message-id", *messageID); err != nil {
			return err
		}
		if err := validateUUIDFlag("--message-id", *messageID); err != nil {
			return err
		}
		client, environment, err := resolveClient(ctx, common)
		if err != nil {
			return usageOrWrap("storekit retention-messaging messages delete", err)
		}
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		if err := client.DeleteMessage(requestCtx, *messageID); err != nil {
			return fmt.Errorf("storekit retention-messaging messages delete: %w", err)
		}
		return printMutation(mutation("message", *messageID, "deleted", environment), *output.Output, *output.Pretty)
	})
}

func defaultsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging defaults", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "defaults",
		ShortUsage: "asc storekit retention-messaging defaults <subcommand> [flags]",
		ShortHelp:  "View, set, and delete product-locale default messages.",
		FlagSet:    fs,
		Subcommands: []*ffcli.Command{
			defaultViewCommand(), defaultSetCommand(), defaultDeleteCommand(),
		},
		UsageFunc: shared.DefaultUsageFunc,
		Exec:      func(ctx context.Context, args []string) error { return flag.ErrHelp },
	}
}

func bindDefaultFlags(fs *flag.FlagSet) (*string, *string) {
	return fs.String("product-id", "", "Subscription product ID"), fs.String("locale", "", "App Store locale, for example en-US")
}

func defaultViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging defaults view", flag.ExitOnError)
	productID, locale := bindDefaultFlags(fs)
	common := bindCommonFlags(fs)
	output := shared.BindOutputFlags(fs)
	return leafCommand("view", "asc storekit retention-messaging defaults view --product-id ID --locale LOCALE --environment ENV [flags]", "View the default retention message.", fs, func(ctx context.Context, args []string) error {
		if err := validateDefaultInvocation(args, *productID, *locale); err != nil {
			return err
		}
		client, _, err := resolveClient(ctx, common)
		if err != nil {
			return usageOrWrap("storekit retention-messaging defaults view", err)
		}
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		result, err := client.GetDefault(requestCtx, *productID, *locale)
		if err != nil {
			return fmt.Errorf("storekit retention-messaging defaults view: %w", err)
		}
		return printOutput(result, *output.Output, *output.Pretty, []string{"Message ID"}, [][]string{{result.MessageIdentifier}})
	})
}

func defaultSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging defaults set", flag.ExitOnError)
	productID, locale := bindDefaultFlags(fs)
	messageID := fs.String("message-id", "", "Approved retention message UUID")
	common := bindCommonFlags(fs)
	output := shared.BindOutputFlags(fs)
	return leafCommand("set", "asc storekit retention-messaging defaults set --product-id ID --locale LOCALE --message-id UUID --environment ENV [flags]", "Set the default retention message.", fs, func(ctx context.Context, args []string) error {
		if err := validateDefaultInvocation(args, *productID, *locale); err != nil {
			return err
		}
		if err := requireFlag("--message-id", *messageID); err != nil {
			return err
		}
		if err := validateUUIDFlag("--message-id", *messageID); err != nil {
			return err
		}
		client, _, err := resolveClient(ctx, common)
		if err != nil {
			return usageOrWrap("storekit retention-messaging defaults set", err)
		}
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		result, err := client.SetDefault(requestCtx, *productID, *locale, *messageID)
		if err != nil {
			return fmt.Errorf("storekit retention-messaging defaults set: %w", err)
		}
		return printOutput(result, *output.Output, *output.Pretty, []string{"Message ID"}, [][]string{{result.MessageIdentifier}})
	})
}

func defaultDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging defaults delete", flag.ExitOnError)
	productID, locale := bindDefaultFlags(fs)
	confirm := fs.Bool("confirm", false, "Confirm default deletion")
	common := bindCommonFlags(fs)
	output := shared.BindOutputFlags(fs)
	return leafCommand("delete", "asc storekit retention-messaging defaults delete --product-id ID --locale LOCALE --environment ENV --confirm [flags]", "Delete the default retention message.", fs, func(ctx context.Context, args []string) error {
		if err := rejectUnexpectedArgs(args); err != nil {
			return err
		}
		if !*confirm {
			return shared.UsageError("--confirm is required")
		}
		if err := validateDefaultInvocation(nil, *productID, *locale); err != nil {
			return err
		}
		client, environment, err := resolveClient(ctx, common)
		if err != nil {
			return usageOrWrap("storekit retention-messaging defaults delete", err)
		}
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		if err := client.DeleteDefault(requestCtx, *productID, *locale); err != nil {
			return fmt.Errorf("storekit retention-messaging defaults delete: %w", err)
		}
		identifier := strings.TrimSpace(*productID) + "/" + strings.TrimSpace(*locale)
		return printMutation(mutation("default", identifier, "deleted", environment), *output.Output, *output.Pretty)
	})
}

func endpointCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging endpoint", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "endpoint",
		ShortUsage: "asc storekit retention-messaging endpoint <subcommand> [flags]",
		ShortHelp:  "View, set, and delete the realtime endpoint URL.",
		FlagSet:    fs,
		Subcommands: []*ffcli.Command{
			endpointViewCommand(), endpointSetCommand(), endpointDeleteCommand(),
		},
		UsageFunc: shared.DefaultUsageFunc,
		Exec:      func(ctx context.Context, args []string) error { return flag.ErrHelp },
	}
}

func endpointViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging endpoint view", flag.ExitOnError)
	common := bindCommonFlags(fs)
	output := shared.BindOutputFlags(fs)
	return leafCommand("view", "asc storekit retention-messaging endpoint view --environment ENV [flags]", "View the realtime retention endpoint URL.", fs, func(ctx context.Context, args []string) error {
		if err := rejectUnexpectedArgs(args); err != nil {
			return err
		}
		client, _, err := resolveClient(ctx, common)
		if err != nil {
			return usageOrWrap("storekit retention-messaging endpoint view", err)
		}
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		result, err := client.GetRealtimeURL(requestCtx)
		if err != nil {
			return fmt.Errorf("storekit retention-messaging endpoint view: %w", err)
		}
		return printOutput(result, *output.Output, *output.Pretty, []string{"Endpoint URL"}, [][]string{{result.RealtimeURL}})
	})
}

func endpointSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging endpoint set", flag.ExitOnError)
	realtimeURL := fs.String("url", "", "Complete HTTPS Get Retention Message endpoint URL")
	common := bindCommonFlags(fs)
	output := shared.BindOutputFlags(fs)
	return leafCommand("set", "asc storekit retention-messaging endpoint set --url HTTPS_URL --environment ENV [flags]", "Set the realtime retention endpoint URL.", fs, func(ctx context.Context, args []string) error {
		if err := rejectUnexpectedArgs(args); err != nil {
			return err
		}
		if err := requireFlag("--url", *realtimeURL); err != nil {
			return err
		}
		if err := storekitapi.ValidateRealtimeURL(*realtimeURL); err != nil {
			return shared.UsageError("--url " + strings.TrimPrefix(err.Error(), "realtime URL "))
		}
		client, _, err := resolveClient(ctx, common)
		if err != nil {
			return usageOrWrap("storekit retention-messaging endpoint set", err)
		}
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		result, err := client.SetRealtimeURL(requestCtx, *realtimeURL)
		if err != nil {
			return fmt.Errorf("storekit retention-messaging endpoint set: %w", err)
		}
		return printOutput(result, *output.Output, *output.Pretty, []string{"Endpoint URL"}, [][]string{{result.RealtimeURL}})
	})
}

func endpointDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging endpoint delete", flag.ExitOnError)
	confirm := fs.Bool("confirm", false, "Confirm endpoint URL deletion")
	common := bindCommonFlags(fs)
	output := shared.BindOutputFlags(fs)
	return leafCommand("delete", "asc storekit retention-messaging endpoint delete --environment ENV --confirm [flags]", "Delete the realtime retention endpoint URL.", fs, func(ctx context.Context, args []string) error {
		if err := rejectUnexpectedArgs(args); err != nil {
			return err
		}
		if !*confirm {
			return shared.UsageError("--confirm is required")
		}
		client, environment, err := resolveClient(ctx, common)
		if err != nil {
			return usageOrWrap("storekit retention-messaging endpoint delete", err)
		}
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		if err := client.DeleteRealtimeURL(requestCtx); err != nil {
			return fmt.Errorf("storekit retention-messaging endpoint delete: %w", err)
		}
		return printMutation(mutation("endpoint", "", "deleted", environment), *output.Output, *output.Pretty)
	})
}

func performanceCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging performance", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "performance",
		ShortUsage: "asc storekit retention-messaging performance <subcommand> [flags]",
		ShortHelp:  "Start and inspect sandbox endpoint performance tests.",
		LongHelp: `Start and inspect Retention Messaging endpoint performance tests.

Apple exposes these operations only in the sandbox environment.`,
		FlagSet: fs,
		Subcommands: []*ffcli.Command{
			performanceStartCommand(), performanceViewCommand(), performanceWaitCommand(),
		},
		UsageFunc: shared.DefaultUsageFunc,
		Exec:      func(ctx context.Context, args []string) error { return flag.ErrHelp },
	}
}

func performanceStartCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging performance start", flag.ExitOnError)
	originalTransactionID := fs.String("original-transaction-id", "", "Sandbox subscription original transaction ID")
	wait := fs.Bool("wait", false, "Wait until the performance test finishes")
	interval := fs.Duration("interval", 10*time.Second, "Polling interval when --wait is set (minimum 10s)")
	timeout := fs.Duration("timeout", 10*time.Minute, "Maximum wait time")
	common := bindCommonFlags(fs)
	output := shared.BindOutputFlags(fs)
	return leafCommand("start", "asc storekit retention-messaging performance start --original-transaction-id ID --environment sandbox [flags]", "Start a sandbox endpoint performance test.", fs, func(ctx context.Context, args []string) error {
		if err := rejectUnexpectedArgs(args); err != nil {
			return err
		}
		if err := requireFlag("--original-transaction-id", *originalTransactionID); err != nil {
			return err
		}
		if !*wait && flagWasSet(fs, "interval", "timeout") {
			return shared.UsageError("--interval and --timeout require --wait")
		}
		if *wait {
			if err := validateWaitDurations(*interval, *timeout); err != nil {
				return err
			}
		}
		client, err := resolveSandboxClient(ctx, common)
		if err != nil {
			return usageOrWrap("storekit retention-messaging performance start", err)
		}
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		result, err := client.StartPerformanceTest(requestCtx, *originalTransactionID)
		cancel()
		if err != nil {
			return fmt.Errorf("storekit retention-messaging performance start: %w", err)
		}
		if *wait {
			final, err := waitForPerformance(ctx, client, result.RequestID, *interval, *timeout)
			if err != nil {
				return fmt.Errorf("storekit retention-messaging performance start: %w", err)
			}
			return printWaitedPerformanceResult(final, *output.Output, *output.Pretty)
		}
		return printOutput(result, *output.Output, *output.Pretty, []string{"Request ID"}, [][]string{{result.RequestID}})
	})
}

func performanceViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging performance view", flag.ExitOnError)
	requestID := fs.String("request-id", "", "Performance test request ID")
	common := bindCommonFlags(fs)
	output := shared.BindOutputFlags(fs)
	return leafCommand("view", "asc storekit retention-messaging performance view --request-id ID --environment sandbox [flags]", "View a sandbox performance test result.", fs, func(ctx context.Context, args []string) error {
		if err := rejectUnexpectedArgs(args); err != nil {
			return err
		}
		if err := requireFlag("--request-id", *requestID); err != nil {
			return err
		}
		client, err := resolveSandboxClient(ctx, common)
		if err != nil {
			return usageOrWrap("storekit retention-messaging performance view", err)
		}
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		result, err := client.GetPerformanceTestResult(requestCtx, *requestID)
		if err != nil {
			return fmt.Errorf("storekit retention-messaging performance view: %w", err)
		}
		if result.RequestID == "" {
			result.RequestID = strings.TrimSpace(*requestID)
		}
		return printWaitedPerformanceResult(result, *output.Output, *output.Pretty)
	})
}

func performanceWaitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit retention-messaging performance wait", flag.ExitOnError)
	requestID := fs.String("request-id", "", "Performance test request ID")
	interval := fs.Duration("interval", 10*time.Second, "Polling interval (minimum 10s)")
	timeout := fs.Duration("timeout", 10*time.Minute, "Maximum wait time")
	common := bindCommonFlags(fs)
	output := shared.BindOutputFlags(fs)
	return leafCommand("wait", "asc storekit retention-messaging performance wait --request-id ID --environment sandbox [flags]", "Wait for a sandbox performance test to finish.", fs, func(ctx context.Context, args []string) error {
		if err := rejectUnexpectedArgs(args); err != nil {
			return err
		}
		if err := requireFlag("--request-id", *requestID); err != nil {
			return err
		}
		if err := validateWaitDurations(*interval, *timeout); err != nil {
			return err
		}
		client, err := resolveSandboxClient(ctx, common)
		if err != nil {
			return usageOrWrap("storekit retention-messaging performance wait", err)
		}
		result, err := waitForPerformance(ctx, client, *requestID, *interval, *timeout)
		if err != nil {
			return fmt.Errorf("storekit retention-messaging performance wait: %w", err)
		}
		return printWaitedPerformanceResult(result, *output.Output, *output.Pretty)
	})
}

func leafCommand(name, usage, help string, fs *flag.FlagSet, exec func(context.Context, []string) error) *ffcli.Command {
	return &ffcli.Command{Name: name, ShortUsage: usage, ShortHelp: help, FlagSet: fs, UsageFunc: shared.DefaultUsageFunc, Exec: exec}
}

func requireFlag(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return shared.UsageError(name + " is required")
	}
	return nil
}

func validateUUIDFlag(name, value string) error {
	_, err := parseUUIDFlag(name, value)
	return err
}

func parseUUIDFlag(name, value string) (string, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", shared.UsageError(name + " must be a UUID")
	}
	return parsed.String(), nil
}

func usageOrWrap(command string, err error) error {
	message := err.Error()
	if strings.HasPrefix(message, "--") || strings.HasPrefix(message, "environment must") || strings.HasPrefix(message, "performance tests require") || strings.HasPrefix(message, "incomplete StoreKit environment") || strings.HasPrefix(message, "mixed StoreKit authentication") {
		return shared.UsageError(message)
	}
	return fmt.Errorf("%s: %w", command, err)
}

func resolveSandboxClient(ctx context.Context, common commonFlags) (*storekitapi.Client, error) {
	environment, err := resolveEnvironment(common.Environment)
	if err != nil {
		return nil, err
	}
	if environment != storekitapi.Sandbox {
		return nil, fmt.Errorf("performance tests require --environment sandbox")
	}
	client, _, err := resolveClient(ctx, common)
	return client, err
}

func mutation(resource, identifier, action string, environment storekitapi.Environment) storekitapi.MutationResult {
	return storekitapi.MutationResult{Resource: resource, Identifier: strings.TrimSpace(identifier), Action: action, Environment: environment, Success: true}
}

func printMutation(result storekitapi.MutationResult, format string, pretty bool) error {
	return printOutput(result, format, pretty,
		[]string{"Resource", "Identifier", "Action", "Environment", "Success"},
		[][]string{{result.Resource, result.Identifier, result.Action, string(result.Environment), boolString(result.Success)}})
}

func readPNG(path string, size storekitapi.ImageSize) ([]byte, error) {
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return nil, fmt.Errorf("--file: %w", err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("--file must be a valid PNG: %w", err)
	}
	if err := validateImageDimensions(config.Width, config.Height, size); err != nil {
		return nil, err
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("--file must be a valid PNG: %w", err)
	}
	if hasTransparency(decoded) {
		return nil, fmt.Errorf("--file must not contain transparency")
	}
	return data, nil
}

func validateImageDimensions(width, height int, size storekitapi.ImageSize) error {
	switch size {
	case storekitapi.ImageSizeFull:
		if width != 3840 || height < 160 || height > 2160 {
			return fmt.Errorf("--file must be 3840 pixels wide and 160-2160 pixels high for FULL_SIZE (got %dx%d)", width, height)
		}
	case storekitapi.ImageSizeBulletPoint:
		if width != 1024 || height != 1024 {
			return fmt.Errorf("--file must be 1024x1024 pixels for BULLET_POINT (got %dx%d)", width, height)
		}
	}
	return nil
}

func hasTransparency(decoded image.Image) bool {
	bounds := decoded.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := decoded.At(x, y).RGBA()
			if alpha != 0xffff {
				return true
			}
		}
	}
	return false
}

func readMessage(path string) (storekitapi.Message, error) {
	file, err := os.Open(strings.TrimSpace(path))
	if err != nil {
		return storekitapi.Message{}, fmt.Errorf("--file: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	var message storekitapi.Message
	if err := decoder.Decode(&message); err != nil {
		return storekitapi.Message{}, fmt.Errorf("--file must contain a valid retention message JSON object: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return storekitapi.Message{}, fmt.Errorf("--file must contain exactly one JSON object")
	}
	if err := storekitapi.ValidateMessage(message); err != nil {
		return storekitapi.Message{}, fmt.Errorf("--file: %w", err)
	}
	return message, nil
}

func validateDefaultInvocation(args []string, productID, locale string) error {
	if err := rejectUnexpectedArgs(args); err != nil {
		return err
	}
	if err := requireFlag("--product-id", productID); err != nil {
		return err
	}
	return requireFlag("--locale", locale)
}

func validateWaitDurations(interval, timeout time.Duration) error {
	if interval < 10*time.Second {
		return shared.UsageError("--interval must be at least 10s for the sandbox rate limit")
	}
	if timeout <= 0 {
		return shared.UsageError("--timeout must be positive")
	}
	return nil
}

func waitForPerformance(ctx context.Context, client *storekitapi.Client, requestID string, interval, timeout time.Duration) (*storekitapi.PerformanceTestResult, error) {
	waitCtx, cancel := context.WithTimeout(shared.ContextWithoutTimeout(ctx), timeout)
	defer cancel()
	for {
		requestCtx, requestCancel := shared.ContextWithTimeout(waitCtx)
		result, err := client.GetPerformanceTestResult(requestCtx, requestID)
		requestCancel()
		if err != nil {
			return nil, err
		}
		if result.RequestID == "" {
			result.RequestID = strings.TrimSpace(requestID)
		}
		if !strings.EqualFold(result.Result, "PENDING") {
			return result, nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-waitCtx.Done():
			timer.Stop()
			return nil, fmt.Errorf("timed out waiting for performance test %q: %w", requestID, waitCtx.Err())
		case <-timer.C:
		}
	}
}

func printPerformanceResult(result *storekitapi.PerformanceTestResult, format string, pretty bool) error {
	return printOutput(result, format, pretty,
		[]string{"Request ID", "Result", "Success Rate", "Pending"},
		[][]string{{result.RequestID, result.Result, fmt.Sprintf("%d", result.SuccessRate), fmt.Sprintf("%d", result.NumPending)}})
}

func printWaitedPerformanceResult(result *storekitapi.PerformanceTestResult, format string, pretty bool) error {
	if err := printPerformanceResult(result, format, pretty); err != nil {
		return err
	}
	return performanceResultError(result)
}

func performanceResultError(result *storekitapi.PerformanceTestResult) error {
	if strings.EqualFold(result.Result, "FAIL") {
		return fmt.Errorf("performance test %q failed", result.RequestID)
	}
	return nil
}
