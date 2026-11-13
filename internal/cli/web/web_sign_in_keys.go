package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func WebSignInKeysCommand() *ffcli.Command {
	return &ffcli.Command{
		Name: "sign-in-keys", ShortUsage: "asc web sign-in-keys <subcommand> [flags]", ShortHelp: "Create and download Sign in with Apple keys through Developer Portal.", FlagSet: flag.NewFlagSet("web sign-in-keys", flag.ExitOnError), UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{WebSignInKeysListCommand(), WebSignInKeysViewCommand(), WebSignInKeysCreateCommand(), WebSignInKeysDownloadCommand()}, Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func WebSignInKeysListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web sign-in-keys list", flag.ExitOnError)
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{Name: "list", ShortUsage: "asc web sign-in-keys list [flags]", ShortHelp: "List Developer Portal authentication keys and download availability.", LongHelp: "Lists up to 1000 authentication keys, including other services, to inspect existing keys before creating one. JSON preserves Apple's response envelope. This does not download private keys.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc, Exec: func(ctx context.Context, args []string) error {
		if err := validateDeveloperPortalFlags(portalFlags); err != nil {
			return err
		}
		if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
			return shared.UsageError(err.Error())
		}
		session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
		defer cancel()
		if err != nil {
			return err
		}
		result, err := newDeveloperPortalClient(session, portalFlags).ListDeveloperSignInKeys(requestCtx)
		if err != nil {
			return withWebAuthHint(err, "web sign-in-keys list")
		}
		persistDeveloperPortalSession(session)
		rows := [][]string{}
		for _, key := range result.Keys {
			rows = append(rows, []string{key.KeyID, key.KeyName, fmt.Sprint(key.CanDownload)})
		}
		headers := []string{"Key ID", "Name", "Can Download"}
		return shared.PrintOutputWithRenderers(result, *output.Output, *output.Pretty, func() error { asc.RenderTable(headers, rows); return nil }, func() error { asc.RenderMarkdown(headers, rows); return nil })
	}}
}

func WebSignInKeysViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web sign-in-keys view", flag.ExitOnError)
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)
	keyID := fs.String("key-id", "", "Developer Portal authentication key ID")
	return &ffcli.Command{Name: "view", ShortUsage: "asc web sign-in-keys view --key-id ID [flags]", ShortHelp: "Inspect authentication key services and configured bundle IDs.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc, Exec: func(ctx context.Context, args []string) error {
		if strings.TrimSpace(*keyID) == "" {
			return shared.UsageError("--key-id is required")
		}
		if !regexp.MustCompile(`^[A-Za-z0-9]{1,32}$`).MatchString(*keyID) {
			return shared.UsageError("invalid --key-id")
		}
		if err := validateDeveloperPortalFlags(portalFlags); err != nil {
			return err
		}
		if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
			return shared.UsageError(err.Error())
		}
		session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
		defer cancel()
		if err != nil {
			return err
		}
		key, err := newDeveloperPortalClient(session, portalFlags).GetDeveloperSignInKey(requestCtx, *keyID)
		if err != nil {
			return err
		}
		persistDeveloperPortalSession(session)
		headers := []string{"Key ID", "Name", "Can Download"}
		rows := [][]string{{key.KeyID, key.KeyName, fmt.Sprint(key.CanDownload)}}
		return shared.PrintOutputWithRenderers(json.RawMessage(key.Raw), *output.Output, *output.Pretty, func() error { asc.RenderTable(headers, rows); return nil }, func() error { asc.RenderMarkdown(headers, rows); return nil })
	}}
}

var createDeveloperSignInKeyFn = func(ctx context.Context, client *webcore.Client, name, bundleID string) (*webcore.DeveloperSignInKey, error) {
	return client.CreateDeveloperSignInKey(ctx, name, bundleID)
}

var downloadDeveloperSignInKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) ([]byte, error) {
	return client.DownloadDeveloperSignInKey(ctx, keyID)
}

type signInKeyDownloadRecovery interface {
	error
	RecoveryBody() []byte
}

func WebSignInKeysCreateCommand() *ffcli.Command   { return webSignInKeySaveCommand(true) }
func WebSignInKeysDownloadCommand() *ffcli.Command { return webSignInKeySaveCommand(false) }

func webSignInKeySaveCommand(create bool) *ffcli.Command {
	operation := "download"
	if create {
		operation = "create"
	}
	shortUsage := "asc web sign-in-keys download --key-id ID --output-dir DIR --confirm [flags]"
	longHelp := "Requires an authenticated Developer Portal session and macOS or Linux for private file publication. Uses the existing --developer-team selection. Saves AuthKey_<KEY_ID>.p8 with mode 0600; private bytes are never printed. An uncertain create or download is never automatically retried. Inspect list before retrying a failed create."
	fs := flag.NewFlagSet("web sign-in-keys "+operation, flag.ExitOnError)
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)
	outputDir := fs.String("output-dir", "", "Directory for the private P8 file (mode 0600; never overwrite)")
	var name, bundleID, keyID string
	var confirm bool
	if create {
		fs.StringVar(&name, "name", "", "Key display name")
		fs.StringVar(&bundleID, "bundle-id", "", "Primary Sign in with Apple Bundle ID resource ID, not reverse-DNS identifier")
		shortUsage = "asc web sign-in-keys create --name NAME --bundle-id BUNDLE_RESOURCE_ID --output-dir DIR --confirm [flags]"
	} else {
		fs.StringVar(&keyID, "key-id", "", "Developer Portal authentication key ID")
	}
	confirmHelp := "Confirm consuming this one-time Sign in with Apple private key download"
	if create {
		confirmHelp = "Confirm creating this Sign in with Apple private key and consuming its one-time download"
		longHelp += " Creating a new Developer Portal key and consuming its one-time download require --confirm."
	} else {
		longHelp += " Consuming this one-time key download requires --confirm."
	}
	fs.BoolVar(&confirm, "confirm", false, confirmHelp)
	return &ffcli.Command{Name: operation, ShortUsage: shortUsage, ShortHelp: strings.ToUpper(operation[:1]) + operation[1:] + " a Sign in with Apple key and save its one-time P8 privately.", LongHelp: longHelp, FlagSet: fs, UsageFunc: shared.DefaultUsageFunc, Exec: func(ctx context.Context, args []string) error {
		name = strings.TrimSpace(name)
		bundleID = strings.TrimSpace(bundleID)
		keyID = strings.TrimSpace(keyID)
		if create && name == "" {
			return shared.UsageError("--name is required")
		}
		if create && bundleID == "" {
			return shared.UsageError("--bundle-id is required")
		}
		if !create && keyID == "" {
			return shared.UsageError("--key-id is required")
		}
		idPattern := regexp.MustCompile(`^[A-Za-z0-9]{1,32}$`)
		if create && !idPattern.MatchString(bundleID) {
			return shared.UsageError("--bundle-id must be an opaque Bundle ID resource ID")
		}
		if !create && !idPattern.MatchString(keyID) {
			return shared.UsageError("invalid --key-id")
		}
		if strings.TrimSpace(*outputDir) == "" {
			return shared.UsageError("--output-dir is required")
		}
		if !confirm {
			return shared.UsageError("--confirm is required")
		}
		if err := validateDeveloperPortalFlags(portalFlags); err != nil {
			return err
		}
		if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
			return shared.UsageError(err.Error())
		}
		root, err := rootfs.New(strings.TrimSpace(*outputDir))
		if err != nil {
			return err
		}
		defer root.Close()
		if err = root.MkdirAll(".", 0o700); err != nil {
			return err
		}
		// Probe the same conditional publication used below before consuming a key.
		probeName, err := newIndividualAPIKeyStagingName()
		if err != nil {
			return err
		}
		probe, err := root.CreateNewFileAtomicWithIdentity(probeName, nil, 0o600)
		if err != nil {
			return err
		}
		publishedProbe, probeErr := root.ReplaceFileIfSame(probeName, probe, nil, 0o600, false)
		if publishedProbe != nil {
			probe = publishedProbe
		}
		cleanupErr := root.RemoveFileIfSameIdentity(probeName, probe)
		if err = errors.Join(probeErr, cleanupErr); err != nil {
			return fmt.Errorf("verify private-key output publication before contacting Apple: %w", err)
		}
		stagedName, err := newIndividualAPIKeyStagingName()
		if err != nil {
			return err
		}
		stagedPath := filepath.Join(root.Path(), stagedName)
		opened, err := root.OpenRoot()
		if err != nil {
			return err
		}
		defer opened.Close()
		stagedFile, err := secureopen.OpenNewPrivateFileNoFollowInRoot(opened, stagedName, 0o600)
		if err != nil {
			return fmt.Errorf("prepare private recovery file before contacting Apple: %w", err)
		}
		defer stagedFile.Close()
		if err = secureopen.PreparePrivateFile(stagedFile, 0o600); err != nil {
			return err
		}
		if err = errors.Join(stagedFile.Sync(), syncSignInKeyDirectory(opened)); err != nil {
			return fmt.Errorf("prepare durable private recovery file before contacting Apple: %w", err)
		}
		emptyStage, err := root.CaptureFile(stagedName)
		if err != nil {
			return err
		}
		keepStage := false
		defer func() {
			if !keepStage {
				_ = root.RemoveFileIfSameIdentity(stagedName, emptyStage)
			}
		}()

		session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
		defer cancel()
		if err != nil {
			return err
		}
		client := newDeveloperPortalClient(session, portalFlags)
		if create {
			key, createErr := createDeveloperSignInKeyFn(requestCtx, client, name, bundleID)
			if createErr != nil {
				return withWebAuthHint(createErr, "web sign-in-keys create")
			}
			keyID = key.KeyID
		}
		persistDeveloperPortalSession(session)
		fileName := "AuthKey_" + keyID + ".p8"
		reservation, err := root.CreateNewFileAtomicWithIdentity(fileName, nil, 0o600)
		if err != nil {
			return fmt.Errorf("key %s exists, but destination %s could not be reserved; P8 was not downloaded. Recover with sign-in-keys download --key-id %s --output-dir OTHER_DIR --confirm; do not rerun create: %w", keyID, filepath.Join(root.Path(), fileName), keyID, err)
		}
		defer func() { _ = root.RemoveFileIfSameIdentity(fileName, reservation) }()
		p8, err := downloadDeveloperSignInKeyFn(requestCtx, client, keyID)
		if err != nil {
			var recovery signInKeyDownloadRecovery
			if errors.As(err, &recovery) {
				raw := recovery.RecoveryBody()
				if len(raw) > 0 {
					// The one-time response may already be consumed. Preserve the
					// raw bytes privately before returning the validation error.
					keepStage = true
					writeErr := writeSignInKeyStage(stagedFile, raw)
					if writeErr != nil {
						cleanupErr := root.RemoveFileIfSameIdentity(fileName, reservation)
						return errors.Join(
							fmt.Errorf("key %s returned an invalid one-time response; private recovery file at %s may be partial; inspect before retrying; do not retry automatically: %w", keyID, stagedPath, writeErr),
							err,
							cleanupErr,
						)
					}
					if verifyErr := verifySignInKeyStage(root, stagedName, stagedFile); verifyErr != nil {
						cleanupErr := root.RemoveFileIfSameIdentity(fileName, reservation)
						return errors.Join(
							fmt.Errorf("key %s returned an invalid one-time response; inspect recovery path %s before retrying; do not retry automatically: %w", keyID, stagedPath, verifyErr),
							err,
							cleanupErr,
						)
					}
					cleanupErr := root.RemoveFileIfSameIdentity(fileName, reservation)
					return errors.Join(fmt.Errorf("key %s returned an invalid one-time response; raw response retained at %s; inspect before retrying; do not retry automatically: %w", keyID, stagedPath, err), cleanupErr)
				}
			}
			cleanupErr := root.RemoveFileIfSameIdentity(fileName, reservation)
			return errors.Join(fmt.Errorf("key %s exists, but download failed; inspect before retrying: %w", keyID, err), cleanupErr)
		}
		keepStage = true
		writeErr := writeSignInKeyStage(stagedFile, p8)
		if writeErr != nil {
			cleanupErr := root.RemoveFileIfSameIdentity(fileName, reservation)
			return errors.Join(fmt.Errorf("key %s downloaded but writing failed; private recovery file at %s may be partial; inspect before retrying because the one-time download may be consumed: %w", keyID, stagedPath, writeErr), cleanupErr)
		}
		if err = verifySignInKeyStage(root, stagedName, stagedFile); err != nil {
			return fmt.Errorf("key %s downloaded; inspect private recovery file %s before retrying: %w", keyID, stagedPath, err)
		}
		if err = stagedFile.Close(); err != nil {
			return fmt.Errorf("key %s downloaded; inspect private recovery file %s: %w", keyID, stagedPath, err)
		}

		if err = root.RemoveFileIfSameIdentity(fileName, reservation); err != nil {
			return fmt.Errorf("key %s downloaded; destination changed; private recovery copy retained at %s: %w", keyID, stagedPath, err)
		}
		if err = materializeIndividualAPIKey(root, stagedName, fileName); err != nil {
			return fmt.Errorf("key %s downloaded; publication failed; inspect private recovery copy at %s and destination %s before retrying: %w", keyID, stagedPath, filepath.Join(root.Path(), fileName), err)
		}
		if err = syncSignInKeyDirectory(opened); err != nil {
			return fmt.Errorf("key %s saved at %s but directory durability could not be confirmed; preserve this file and do not retry the one-time download: %w", keyID, filepath.Join(root.Path(), fileName), err)
		}
		persistDeveloperPortalSession(session)
		return shared.PrintOutput(&asc.WebSignInKeyReceipt{KeyID: keyID, Name: name, BundleID: bundleID, P8Path: filepath.Join(root.Path(), fileName)}, *output.Output, *output.Pretty)
	}}
}

func writeSignInKeyStage(stagedFile *os.File, p8 []byte) error {
	written, err := stagedFile.Write(p8)
	if err == nil && written != len(p8) {
		err = io.ErrShortWrite
	}
	return errors.Join(err, stagedFile.Sync())
}

func verifySignInKeyStage(root rootfs.Root, stagedName string, stagedFile *os.File) error {
	// Verify the named recovery file still refers to our open private inode.
	saved, err := root.OpenFile(stagedName)
	if err != nil {
		return err
	}
	savedInfo, statErr := saved.Stat()
	openInfo, openErr := stagedFile.Stat()
	closeErr := saved.Close()
	if err := errors.Join(statErr, openErr, closeErr); err != nil {
		return err
	}
	if !os.SameFile(savedInfo, openInfo) {
		return fmt.Errorf("recovery path was replaced")
	}
	return nil
}

func syncSignInKeyDirectory(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}
