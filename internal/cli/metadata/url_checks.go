package metadata

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/metadataurl"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

var newMetadataURLChecker = metadataurl.NewChecker

type metadataURLTarget struct {
	rawURL  string
	scope   string
	file    string
	locale  string
	version string
	field   string
	label   string
}

type metadataURLCheckOutcome struct {
	result metadataurl.Result
	err    error
}

func metadataURLTargets(scope, file, locale, version string, fields []metadataURLField) []metadataURLTarget {
	targets := make([]metadataURLTarget, 0, len(fields))
	for _, field := range fields {
		rawURL := strings.TrimSpace(field.value)
		if rawURL == "" || !validation.IsValidHTTPURL(rawURL) {
			continue
		}
		targets = append(targets, metadataURLTarget{
			rawURL:  rawURL,
			scope:   scope,
			file:    file,
			locale:  locale,
			version: version,
			field:   field.field,
			label:   field.label,
		})
	}
	return targets
}

func metadataURLCheckIssues(ctx context.Context, checker metadataurl.Checker, targets []metadataURLTarget) ([]ValidateIssue, error) {
	urls := make([]string, 0, len(targets))
	for _, target := range targets {
		urls = append(urls, target.rawURL)
	}
	outcomes, err := metadataurl.CheckAll(ctx, checker, urls)
	if err != nil {
		return nil, err
	}

	issues := make([]ValidateIssue, 0, len(targets))
	for _, target := range targets {
		checked := outcomes[target.rawURL]
		outcome := metadataURLCheckOutcome{result: checked.Result, err: checked.Err}
		for _, message := range metadataURLCheckMessages(target, outcome) {
			issues = append(issues, ValidateIssue{
				Scope:    target.scope,
				File:     target.file,
				Locale:   target.locale,
				Version:  target.version,
				Field:    target.field,
				Severity: issueSeverityWarning,
				Message:  message,
			})
		}
	}
	return issues, nil
}

func metadataURLCheckMessages(target metadataURLTarget, outcome metadataURLCheckOutcome) []string {
	if outcome.err != nil {
		reason := "request failed"
		if errors.Is(outcome.err, metadataurl.ErrUnsafeTarget) {
			reason = "target is not a public internet address"
		} else if errors.Is(outcome.err, context.DeadlineExceeded) || isTimeoutError(outcome.err) {
			reason = "request timed out"
		}
		return []string{fmt.Sprintf("%s could not be checked: %s", target.label, reason)}
	}
	if outcome.result.FinalURL == nil {
		return []string{fmt.Sprintf("%s could not be checked: request failed", target.label)}
	}

	initialURL, err := url.Parse(target.rawURL)
	if err != nil {
		return nil
	}
	messages := make([]string, 0, 2)
	initialHost := strings.ToLower(initialURL.Hostname())
	finalHost := strings.ToLower(outcome.result.FinalURL.Hostname())
	if outcome.result.RedirectedHost || initialHost != finalHost {
		if outcome.result.RedirectedHost && initialHost != "" && initialHost == finalHost {
			messages = append(messages, fmt.Sprintf("%s redirects through a different host before returning to %s", target.label, initialHost))
		} else {
			messages = append(messages, fmt.Sprintf("%s redirects to a different host (%s -> %s)", target.label, initialHost, finalHost))
		}
	}
	if outcome.result.StatusCode < http.StatusOK || outcome.result.StatusCode >= http.StatusMultipleChoices {
		return append(messages, fmt.Sprintf("%s returned HTTP %d", target.label, outcome.result.StatusCode))
	}
	finalPath := outcome.result.FinalURL.EscapedPath()
	if (finalPath == "" || finalPath == "/") && outcome.result.FinalURL.RawQuery == "" && outcome.result.FinalURL.Fragment == "" {
		messages = append(messages, fmt.Sprintf("%s resolves to a site root instead of a dedicated page", target.label))
	}
	return messages
}

func isTimeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
