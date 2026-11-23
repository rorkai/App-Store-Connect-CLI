package web

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

type webReviewShowThreadOptions struct {
	AppID     string
	ThreadID  string
	OutDir    string
	Pattern   string
	Overwrite bool
	PlainText bool
	Output    shared.OutputFlags
}

// findReviewThreadUnderApp returns the thread only when it appears exactly
// once among the app's threads, so an app selector cannot be paired with
// another app's thread. requireWebReviewThreadUnderApp applies the same check
// before Resolution Center writes.
func findReviewThreadUnderApp(threads []webcore.ResolutionCenterThread, command, appID, threadID string) (webcore.ResolutionCenterThread, error) {
	var found webcore.ResolutionCenterThread
	matching := 0
	for _, thread := range threads {
		if strings.TrimSpace(thread.ID) == threadID {
			found = thread
			matching++
		}
	}
	if matching == 0 {
		return webcore.ResolutionCenterThread{}, fmt.Errorf("%s refused: thread %q was not found under app %q", command, threadID, appID)
	}
	if matching > 1 {
		return webcore.ResolutionCenterThread{}, fmt.Errorf("%s refused: thread %q appeared more than once under app %q", command, threadID, appID)
	}
	return found, nil
}

// runWebReviewShowThread shows one app-scoped thread with its messages,
// rejections, and attachments, whichever review submission it belongs to.
func runWebReviewShowThread(ctx context.Context, d *shared.Deps, client *webcore.Client, opts webReviewShowThreadOptions) error {
	appThreads, err := withWebSpinnerValueWith(d, "Loading app resolution center threads", func() ([]webcore.ResolutionCenterThread, error) {
		return client.ListResolutionCenterThreadsByApp(ctx, opts.AppID)
	})
	if err != nil {
		return withWebAuthHint(err, "web review show")
	}
	thread, err := findReviewThreadUnderApp(appThreads, "web review show", opts.AppID, opts.ThreadID)
	if err != nil {
		return err
	}

	var (
		threadDetails      []reviewThreadDetails
		attachmentsWithURL []webcore.ReviewAttachment
	)
	err = withWebSpinnerWith(d, "Loading review thread and attachments", func() error {
		var err error
		threadDetails, attachmentsWithURL, err = buildThreadDetails(ctx, client, []webcore.ResolutionCenterThread{thread}, opts.PlainText)
		return err
	})
	if err != nil {
		return withWebAuthHint(err, "web review show")
	}

	outDirKey := strings.TrimSpace(thread.ReviewSubmissionID)
	if outDirKey == "" {
		outDirKey = opts.ThreadID
	}
	outDirResolved := resolveShowOutDir(opts.AppID, outDirKey, opts.OutDir)
	var (
		downloads        []reviewAttachmentDownloadResult
		downloadFailures []string
	)
	err = withWebSpinnerWith(d, "Downloading review attachments", func() error {
		var err error
		downloads, downloadFailures, err = downloadAttachmentsForShow(
			ctx,
			d,
			client,
			attachmentsWithURL,
			func(ctx context.Context) ([]webcore.ReviewAttachment, error) {
				return client.ListReviewAttachmentsByThread(ctx, opts.ThreadID, true)
			},
			outDirResolved,
			opts.Pattern,
			opts.Overwrite,
		)
		return err
	})
	if err != nil {
		return err
	}

	payload := reviewShowOutput{
		AppID:            opts.AppID,
		Selection:        "thread",
		Threads:          threadDetails,
		AppThreads:       appThreadsOutsideSubmission(appThreads, threadDetails),
		Attachments:      redactAttachmentURLs(attachmentsWithURL),
		OutputDirectory:  outDirResolved,
		Downloads:        downloads,
		DownloadFailures: downloadFailures,
	}
	if len(payload.Downloads) == 0 {
		payload.OutputDirectory = ""
	}
	if err := d.PrintOutputWithRenderers(
		payload,
		*opts.Output.Output,
		*opts.Output.Pretty,
		func() error { return renderReviewShowTable(d, payload) },
		func() error { return renderReviewShowMarkdown(d, payload) },
	); err != nil {
		return err
	}
	if len(payload.DownloadFailures) > 0 {
		return fmt.Errorf("review show completed with %d download failure(s)", len(payload.DownloadFailures))
	}
	return nil
}
