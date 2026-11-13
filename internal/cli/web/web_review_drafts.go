package web

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const webReviewDraftBodyMaxBytes int64 = 1 << 20

// WebReviewDraftsCommand groups the unsent Resolution Center
// draft operations. The send path remains exclusively on web review reply.
func WebReviewDraftsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review drafts", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "drafts",
		ShortUsage: "asc web review drafts <subcommand> [flags]",
		ShortHelp:  "Manage unsent Resolution Center drafts.",
		LongHelp: `WEB SESSION WORKFLOWS

Create, update, or delete the unsent draft that belongs to one App Store
Connect Resolution Center thread. These commands never send a message and do
not support attachments. Use "asc web review threads --app APP_ID --drafts" to
inspect the current draft before choosing an operation.

Subcommands:
  create  Create one unsent draft
  update  Replace the body of one existing unsent draft
  delete  Delete one existing unsent draft

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebReviewDraftCreateCommand(),
			WebReviewDraftUpdateCommand(),
			WebReviewDraftDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebReviewDraftCreateCommand creates one unsent Resolution Center draft.
func WebReviewDraftCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review drafts create", flag.ExitOnError)
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	threadID := fs.String("thread-id", "", "Resolution Center thread ID")
	message := fs.String("message", "", "Draft message body")
	bodyFile := fs.String("body-file", "", "Read the draft message body from a regular file")
	confirm := fs.Bool("confirm", false, "Confirm creating the unsent draft")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc web review drafts create --app APP_ID --thread-id THREAD_ID (--message MESSAGE | --body-file FILE) --confirm [flags]",
		ShortHelp:  "Create an unsent Resolution Center draft.",
		LongHelp: `Create one unsent draft on an app-scoped Resolution Center thread.

Exactly one of --message or --body-file is required. The body is preserved
verbatim after blank-body validation. --confirm is required; this command does
not send the draft or support attachments.

Example:
  asc web review drafts create --app "APP_ID" --thread-id "THREAD_ID" --message "We updated the demo account." --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateWebReviewDraftOutput(output); err != nil {
				return err
			}
			app, thread, err := validateWebReviewDraftIDs(*appID, *threadID)
			if err != nil {
				return err
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			body, err := readWebReviewDraftBody(fs, *message, *bodyFile)
			if err != nil {
				return err
			}
			return executeWebReviewDraftMutation(ctx, authFlags, output, webReviewDraftMutation{
				Action:   "created",
				AppID:    app,
				ThreadID: thread,
				Body:     body,
			})
		},
	}
}

// WebReviewDraftUpdateCommand updates one existing unsent Resolution Center draft.
func WebReviewDraftUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review drafts update", flag.ExitOnError)
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	threadID := fs.String("thread-id", "", "Resolution Center thread ID")
	draftID := fs.String("draft-id", "", "Resolution Center draft message ID")
	message := fs.String("message", "", "Draft message body")
	bodyFile := fs.String("body-file", "", "Read the draft message body from a regular file")
	confirm := fs.Bool("confirm", false, "Confirm updating the unsent draft")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc web review drafts update --app APP_ID --thread-id THREAD_ID --draft-id DRAFT_ID (--message MESSAGE | --body-file FILE) --confirm [flags]",
		ShortHelp:  "Update an unsent Resolution Center draft.",
		LongHelp: `Replace the body of one existing unsent draft on an app-scoped
Resolution Center thread.

Exactly one of --message or --body-file is required. The body is preserved
verbatim after blank-body validation. --confirm is required; this command does
not send the draft or support attachments.

Example:
  asc web review drafts update --app "APP_ID" --thread-id "THREAD_ID" --draft-id "DRAFT_ID" --message "The updated demo account is ready." --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateWebReviewDraftOutput(output); err != nil {
				return err
			}
			app, thread, err := validateWebReviewDraftIDs(*appID, *threadID)
			if err != nil {
				return err
			}
			resolvedDraftID := strings.TrimSpace(*draftID)
			if resolvedDraftID == "" {
				return shared.UsageError("--draft-id is required")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			body, err := readWebReviewDraftBody(fs, *message, *bodyFile)
			if err != nil {
				return err
			}
			return executeWebReviewDraftMutation(ctx, authFlags, output, webReviewDraftMutation{
				Action:   "updated",
				AppID:    app,
				ThreadID: thread,
				DraftID:  resolvedDraftID,
				Body:     body,
			})
		},
	}
}

// WebReviewDraftDeleteCommand deletes one existing unsent Resolution Center draft.
func WebReviewDraftDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review drafts delete", flag.ExitOnError)
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	threadID := fs.String("thread-id", "", "Resolution Center thread ID")
	draftID := fs.String("draft-id", "", "Resolution Center draft message ID")
	confirm := fs.Bool("confirm", false, "Confirm deleting the unsent draft")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc web review drafts delete --app APP_ID --thread-id THREAD_ID --draft-id DRAFT_ID --confirm [flags]",
		ShortHelp:  "Delete an unsent Resolution Center draft.",
		LongHelp: `Delete one existing unsent draft from an app-scoped Resolution
Center thread. --confirm is required. This command never sends a message and
does not support attachments.

Example:
  asc web review drafts delete --app "APP_ID" --thread-id "THREAD_ID" --draft-id "DRAFT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateWebReviewDraftOutput(output); err != nil {
				return err
			}
			app, thread, err := validateWebReviewDraftIDs(*appID, *threadID)
			if err != nil {
				return err
			}
			resolvedDraftID := strings.TrimSpace(*draftID)
			if resolvedDraftID == "" {
				return shared.UsageError("--draft-id is required")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			return executeWebReviewDraftMutation(ctx, authFlags, output, webReviewDraftMutation{
				Action:   "deleted",
				AppID:    app,
				ThreadID: thread,
				DraftID:  resolvedDraftID,
			})
		},
	}
}

type webReviewDraftMutation struct {
	Action   string
	AppID    string
	ThreadID string
	DraftID  string
	Body     string
}

func validateWebReviewDraftOutput(output shared.OutputFlags) error {
	if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
		return shared.UsageError(err.Error())
	}
	return nil
}

func validateWebReviewDraftIDs(appID, threadID string) (string, string, error) {
	app := strings.TrimSpace(shared.ResolveAppID(appID))
	if app == "" {
		return "", "", shared.UsageError("--app is required (or set ASC_APP_ID)")
	}
	thread := strings.TrimSpace(threadID)
	if thread == "" {
		return "", "", shared.UsageError("--thread-id is required")
	}
	return app, thread, nil
}

func readWebReviewDraftBody(fs *flag.FlagSet, message, bodyFile string) (string, error) {
	messageSet := webReviewDraftFlagProvided(fs, "message")
	bodyFileSet := webReviewDraftFlagProvided(fs, "body-file")
	switch {
	case messageSet && bodyFileSet:
		return "", shared.UsageError("--message and --body-file cannot be combined")
	case !messageSet && !bodyFileSet:
		return "", shared.UsageError("exactly one of --message or --body-file is required")
	case messageSet:
		if !utf8.ValidString(message) {
			return "", shared.UsageError("--message must be valid UTF-8")
		}
		if strings.TrimSpace(message) == "" {
			return "", shared.UsageError("--message must not be empty")
		}
		return message, nil
	}

	path := strings.TrimSpace(bodyFile)
	if path == "" {
		return "", shared.UsageError("--body-file must not be empty")
	}
	if path == "-" {
		return "", shared.UsageError("--body-file does not support stdin")
	}
	file, err := rootfs.OpenFile(path)
	if err != nil {
		return "", fmt.Errorf("read --body-file: %w", err)
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, webReviewDraftBodyMaxBytes+1))
	if err != nil {
		return "", fmt.Errorf("read --body-file: %w", err)
	}
	if int64(len(body)) > webReviewDraftBodyMaxBytes {
		return "", fmt.Errorf("--body-file exceeds the %d-byte size limit", webReviewDraftBodyMaxBytes)
	}
	if !utf8.Valid(body) {
		return "", shared.UsageError("--body-file must contain valid UTF-8")
	}
	messageBody := string(body)
	if strings.TrimSpace(messageBody) == "" {
		return "", shared.UsageError("--body-file must contain a non-empty message body")
	}
	return messageBody, nil
}

func webReviewDraftFlagProvided(fs *flag.FlagSet, name string) bool {
	provided := false
	fs.Visit(func(flag *flag.Flag) {
		if flag.Name == name {
			provided = true
		}
	})
	return provided
}

func executeWebReviewDraftMutation(ctx context.Context, authFlags webSessionFlags, output shared.OutputFlags, mutation webReviewDraftMutation) error {
	operation := webReviewDraftOperation(mutation.Action)
	session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
	defer cancel()
	if err != nil {
		return withWebAuthHint(err, "web review drafts "+operation)
	}
	client := newWebClientFn(session)

	if err := preflightWebReviewDraftMutation(requestCtx, client, mutation); err != nil {
		return err
	}

	var draft *webcore.ResolutionCenterDraftMessage
	switch mutation.Action {
	case "created":
		err = withWebSpinner("Creating Resolution Center draft", func() error {
			var err error
			draft, err = client.CreateResolutionCenterDraftMessage(requestCtx, mutation.ThreadID, mutation.Body)
			return err
		})
	case "updated":
		err = withWebSpinner("Updating Resolution Center draft", func() error {
			var err error
			draft, err = client.UpdateResolutionCenterDraftMessage(requestCtx, mutation.DraftID, mutation.Body)
			return err
		})
	case "deleted":
		err = withWebSpinner("Deleting Resolution Center draft", func() error {
			return client.DeleteResolutionCenterDraftMessage(requestCtx, mutation.DraftID)
		})
	default:
		return fmt.Errorf("unsupported web review draft action %q", mutation.Action)
	}
	if err != nil {
		return webReviewMutationError(err, "web review drafts "+operation)
	}

	if mutation.Action == "created" || mutation.Action == "updated" {
		if draft == nil || strings.TrimSpace(draft.ID) == "" {
			return fmt.Errorf("web review drafts %s failed: response returned no draft id; outcome may be ambiguous and must not be retried automatically", mutation.Action)
		}
		if mutation.Action == "updated" && strings.TrimSpace(draft.ID) != mutation.DraftID {
			return fmt.Errorf("web review drafts update failed: response draft id %q did not match requested draft %q; do not retry automatically", strings.TrimSpace(draft.ID), mutation.DraftID)
		}
		mutation.DraftID = strings.TrimSpace(draft.ID)
	}

	verified, err := verifyWebReviewDraftMutation(requestCtx, client, mutation)
	if err != nil {
		return err
	}
	result := &asc.WebReviewDraftResult{
		AppID:    mutation.AppID,
		ThreadID: mutation.ThreadID,
		DraftID:  mutation.DraftID,
		Action:   mutation.Action,
		Verified: verified,
	}
	if err := shared.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
		return fmt.Errorf("web review drafts %s was verified, but receipt output failed; do not retry automatically: %w", operation, err)
	}
	return nil
}

func webReviewDraftOperation(action string) string {
	switch action {
	case "created":
		return "create"
	case "updated":
		return "update"
	case "deleted":
		return "delete"
	default:
		return action
	}
}

func preflightWebReviewDraftMutation(ctx context.Context, client *webcore.Client, mutation webReviewDraftMutation) error {
	operation := webReviewDraftOperation(mutation.Action)
	threads, err := client.ListResolutionCenterThreadsByApp(ctx, mutation.AppID)
	if err != nil {
		return withWebAuthHint(err, "web review drafts "+operation)
	}
	matching := 0
	for _, thread := range threads {
		if strings.TrimSpace(thread.ID) == mutation.ThreadID {
			matching++
		}
	}
	if matching == 0 {
		return fmt.Errorf("web review drafts %s refused: thread %q was not found under app %q", mutation.Action, mutation.ThreadID, mutation.AppID)
	}
	if matching > 1 {
		return fmt.Errorf("web review drafts %s refused: thread %q appeared more than once under app %q", mutation.Action, mutation.ThreadID, mutation.AppID)
	}

	existing, err := client.GetResolutionCenterDraftMessage(ctx, mutation.ThreadID, false)
	if err != nil {
		return withWebAuthHint(err, "web review drafts "+operation)
	}
	switch mutation.Action {
	case "created":
		if existing != nil {
			draftID := strings.TrimSpace(existing.ID)
			if draftID == "" {
				return fmt.Errorf("web review drafts create refused: thread %q already has a draft with no resource id", mutation.ThreadID)
			}
			return fmt.Errorf("web review drafts create refused: thread %q already has draft %q", mutation.ThreadID, draftID)
		}
	case "updated", "deleted":
		if existing == nil || strings.TrimSpace(existing.ID) == "" {
			return fmt.Errorf("web review drafts %s refused: thread %q has no draft %q", mutation.Action, mutation.ThreadID, mutation.DraftID)
		}
		if strings.TrimSpace(existing.ID) != mutation.DraftID {
			return fmt.Errorf("web review drafts %s refused: thread %q has draft %q, not requested draft %q", mutation.Action, mutation.ThreadID, strings.TrimSpace(existing.ID), mutation.DraftID)
		}
	}
	return nil
}

func verifyWebReviewDraftMutation(ctx context.Context, client *webcore.Client, mutation webReviewDraftMutation) (bool, error) {
	operation := webReviewDraftOperation(mutation.Action)
	draft, err := client.GetResolutionCenterDraftMessage(ctx, mutation.ThreadID, false)
	if err != nil {
		return false, fmt.Errorf("web review drafts %s was applied but post-read verification failed; do not retry automatically: %w", operation, err)
	}
	if mutation.Action == "deleted" {
		if draft != nil {
			return false, fmt.Errorf("web review drafts %s was applied but draft %q still exists after post-read; do not retry automatically", operation, mutation.DraftID)
		}
		return true, nil
	}
	if draft == nil || strings.TrimSpace(draft.ID) == "" {
		return false, fmt.Errorf("web review drafts %s was applied but post-read returned no draft; do not retry automatically", operation)
	}
	if strings.TrimSpace(draft.ID) != mutation.DraftID {
		return false, fmt.Errorf("web review drafts %s was applied but post-read returned draft %q instead of %q; do not retry automatically", operation, strings.TrimSpace(draft.ID), mutation.DraftID)
	}
	if draft.MessageBody != mutation.Body {
		return false, fmt.Errorf("web review drafts %s was applied but post-read body did not match; do not retry automatically", operation)
	}
	return true, nil
}
