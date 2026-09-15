package xcode

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	xcconfigAssignmentPattern = regexp.MustCompile(`^(\s*)([A-Za-z_][A-Za-z0-9_]*(?:\[[^\]\r\n]+\])*)(\s*)(\+=|\?=|=)(\s*)(.*?)([ \t]*)$`)
	xcconfigIncludePattern    = regexp.MustCompile(`^\s*#include(\?)?\s+"([^"]+)"\s*$`)
)

type xcconfigAssignment struct {
	lineIndex     int
	key           string
	baseKey       string
	value         string
	operator      string
	quote         string
	operatorStart int
	operatorEnd   int
	valueStart    int
	valueEnd      int
	continued     bool
}

type xcconfigInclude struct {
	lineIndex int
	path      string
	optional  bool
}

type xcconfigDocument struct {
	lines       []string
	assignments []xcconfigAssignment
	includes    []xcconfigInclude
}

// xcconfigSourceGraphLimitError reports that a bounded collector found a new
// source after its configured file budget was exhausted. The typed error lets
// signing-plan consumers distinguish an incomplete source graph from an
// ordinary read or parse failure, including when the error is wrapped while a
// configuration is being attributed to a target.
type xcconfigSourceGraphLimitError struct {
	path  string
	limit int
	err   error
}

func (e *xcconfigSourceGraphLimitError) Error() string {
	message := fmt.Sprintf("signing plan source graph contains more than %d files", e.limit)
	if e.path != "" {
		message = fmt.Sprintf("%s at %s", message, e.path)
	}
	if e.err != nil {
		return fmt.Sprintf("%s: %v", message, e.err)
	}
	return message
}

func (e *xcconfigSourceGraphLimitError) Unwrap() error {
	return e.err
}

func newXCConfigSourceGraphLimitError(path string, limit int, err error) error {
	return &xcconfigSourceGraphLimitError{path: path, limit: limit, err: err}
}

func isXCConfigSourceGraphLimitError(err error) bool {
	var limitErr *xcconfigSourceGraphLimitError
	return errors.As(err, &limitErr)
}

type xcconfigResolvedValue struct {
	value            string
	path             string
	found            bool
	exact            bool
	missingInherited bool
	conditionals     []xcconfigConditionalValue
}

type xcconfigConditionalValue struct {
	key      string
	value    string
	operator string
	path     string
}

func parseXCConfig(data []byte) (xcconfigDocument, error) {
	lines := splitLinesPreservingEndings(string(data))
	document := xcconfigDocument{lines: lines}
	inBlockComment := false

	for index := 0; index < len(lines); index++ {
		line := lines[index]
		body := strings.TrimSuffix(line, "\n")
		body = strings.TrimSuffix(body, "\r")
		masked, nextInBlock, nextQuote := maskXCConfigCommentsState(body, inBlockComment, 0)
		inBlockComment = nextInBlock

		if matches := xcconfigIncludePattern.FindStringSubmatch(masked); matches != nil {
			document.includes = append(document.includes, xcconfigInclude{
				lineIndex: index,
				path:      matches[2],
				optional:  matches[1] == "?",
			})
			continue
		}

		indices := xcconfigAssignmentPattern.FindStringSubmatchIndex(masked)
		if indices == nil {
			continue
		}
		key := masked[indices[4]:indices[5]]
		operatorStart, operatorEnd := indices[8], indices[9]
		valueStart, valueEnd := indices[12], indices[13]
		joined := body[valueStart:valueEnd]
		endIndex := index
		continuationInBlock := nextInBlock
		continuationQuote := nextQuote
		logical, _ := maskXCConfigComments(joined, false)
		value, quote, err := parseXCConfigValue(logical)
		for err != nil && xcconfigValueHasLineContinuation(joined) && endIndex+1 < len(lines) {
			endIndex++
			nextBody := strings.TrimSuffix(lines[endIndex], "\n")
			nextBody = strings.TrimSuffix(nextBody, "\r")
			nextMasked, nextContinuationInBlock, nextContinuationQuote := maskXCConfigCommentsState(nextBody, continuationInBlock, continuationQuote)
			joined = trimXCConfigLineContinuation(joined) + nextMasked
			continuationInBlock = nextContinuationInBlock
			continuationQuote = nextContinuationQuote
			logical, _ = maskXCConfigComments(joined, false)
			value, quote, err = parseXCConfigValue(logical)
		}
		if err != nil {
			return xcconfigDocument{}, fmt.Errorf("xcconfig line %d: %w", index+1, err)
		}
		document.assignments = append(document.assignments, xcconfigAssignment{
			lineIndex:     index,
			key:           key,
			baseKey:       xcconfigBaseKey(key),
			value:         value,
			operator:      body[operatorStart:operatorEnd],
			quote:         quote,
			operatorStart: operatorStart,
			operatorEnd:   operatorEnd,
			valueStart:    valueStart,
			valueEnd:      valueEnd,
			continued:     endIndex > index || xcconfigValueHasLineContinuation(masked[valueStart:valueEnd]),
		})
		inBlockComment = continuationInBlock
		index = endIndex
	}

	if inBlockComment {
		return xcconfigDocument{}, fmt.Errorf("unterminated block comment in xcconfig")
	}
	return document, nil
}

func xcconfigValueHasLineContinuation(value string) bool {
	trimmed := strings.TrimRight(value, " \t")
	backslashes := 0
	for index := len(trimmed) - 1; index >= 0 && trimmed[index] == '\\'; index-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func trimXCConfigLineContinuation(value string) string {
	trimmed := strings.TrimRight(value, " \t")
	if !xcconfigValueHasLineContinuation(trimmed) {
		return value
	}
	return strings.TrimSuffix(trimmed, "\\")
}

func parseXCConfigValue(raw string) (string, string, error) {
	value := strings.TrimSpace(raw)
	if err := validateXCConfigValueQuotes(value); err != nil {
		return "", "", err
	}
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		decoded, err := decodeXCConfigQuotedValue(value[1:len(value)-1], value[0])
		if err != nil {
			return "", "", err
		}
		return decoded, string(value[0]), nil
	}
	return value, "", nil
}

// decodeXCConfigQuotedValue reverses the escaping emitted by
// quoteXCConfigValue. Backslashes are significant only inside a quoted value:
// a doubled backslash represents one literal backslash, and a backslash before
// the matching delimiter represents that delimiter. Unquoted values retain
// their existing literal backslashes and continuation behavior.
func decodeXCConfigQuotedValue(value string, quote byte) (string, error) {
	var decoded strings.Builder
	decoded.Grow(len(value))
	for index := 0; index < len(value); index++ {
		if value[index] != '\\' {
			decoded.WriteByte(value[index])
			continue
		}
		if index+1 >= len(value) {
			return "", fmt.Errorf("dangling escape in quoted xcconfig value")
		}
		next := value[index+1]
		if next != '\\' && next != quote {
			// Version and signing parsers share this decoder. Preserve
			// generic escapes such as `\ ` so an unrelated assignment cannot
			// abort the whole document.
			decoded.WriteByte('\\')
			decoded.WriteByte(next)
			index++
			continue
		}
		decoded.WriteByte(next)
		index++
	}
	return decoded.String(), nil
}

func validateXCConfigValueQuotes(value string) error {
	var quote byte
	escaped := false
	for index := 0; index < len(value); index++ {
		character := value[index]
		if quote == 0 {
			if (character == '"' || character == '\'') && xcconfigQuoteStartsAt(value, index) {
				quote = character
			}
			continue
		}
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' {
			escaped = true
			continue
		}
		if character == quote {
			quote = 0
		}
	}
	if quote != 0 {
		return fmt.Errorf("unterminated quote %q in xcconfig value", string(quote))
	}
	return nil
}

func xcconfigQuoteStartsAt(value string, index int) bool {
	if index == 0 {
		return true
	}
	switch value[index-1] {
	case ' ', '\t', '=':
		return true
	default:
		return false
	}
}

func splitLinesPreservingEndings(value string) []string {
	if value == "" {
		return []string{""}
	}
	lines := strings.SplitAfter(value, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func maskXCConfigComments(line string, inBlockComment bool) (string, bool) {
	masked, nextInBlock, _ := maskXCConfigCommentsState(line, inBlockComment, 0)
	return masked, nextInBlock
}

func maskXCConfigCommentsState(line string, inBlockComment bool, inQuote byte) (string, bool, byte) {
	masked := []byte(line)
	escaped := false

	for index := 0; index < len(masked); index++ {
		if inBlockComment {
			masked[index] = ' '
			if index+1 < len(masked) && line[index] == '*' && line[index+1] == '/' {
				masked[index+1] = ' '
				index++
				inBlockComment = false
			}
			continue
		}

		character := line[index]
		if inQuote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if character == '\\' {
				escaped = true
				continue
			}
			if character == inQuote {
				inQuote = 0
			}
			continue
		}

		if (character == '"' || character == '\'') && xcconfigQuoteStartsAt(line, index) {
			inQuote = character
			continue
		}
		if index+1 >= len(masked) {
			continue
		}
		if line[index:index+2] == "//" {
			for rest := index; rest < len(masked); rest++ {
				masked[rest] = ' '
			}
			break
		}
		if line[index:index+2] == "/*" {
			masked[index] = ' '
			masked[index+1] = ' '
			index++
			inBlockComment = true
		}
	}
	return string(masked), inBlockComment, inQuote
}

func xcconfigBaseKey(key string) string {
	if index := strings.Index(key, "["); index >= 0 {
		return key[:index]
	}
	return key
}

func resolveXCConfigInclude(containingPath string, include xcconfigInclude) (string, error) {
	if strings.Contains(include.path, "$(") || strings.Contains(include.path, "${") {
		return "", fmt.Errorf("xcconfig include contains unresolved build setting: %s", include.path)
	}
	path := include.path
	if filepath.Ext(path) == "" {
		path += ".xcconfig"
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(containingPath), path)
	}
	return filepath.Clean(path), nil
}

func collectXCConfigFiles(root string) ([]string, error) {
	return collectXCConfigFilesWithReader(root, os.ReadFile, nil)
}

// collectStableXCConfigFiles walks a stable version command's include graph
// with per-directory case semantics. Stable commands retain ordinary
// os.Stat/os.ReadFile behavior (including selected symlinks), but a Windows
// case-sensitive directory must not collapse two case-distinct include paths
// into one traversal key.
func collectStableXCConfigFiles(root string) ([]string, error) {
	var identify func(string) (os.FileInfo, error)
	if xcconfigUsesIdentityTraversal() {
		identify = os.Stat
	}
	return collectXCConfigFilesWithHooksAndIdentity(root, os.ReadFile, identify)
}

// collectXCConfigFilesWithReader walks an xcconfig include graph using the
// caller's reader and authorization hook. Security-sensitive callers should
// authorize every path before the reader or existence check touches it.
func collectXCConfigFilesWithReader(root string, read func(string) ([]byte, error), authorize func(string) error) ([]string, error) {
	return collectXCConfigFilesWithHooks(root, read, authorize, nil, nil)
}

// collectXCConfigFilesWithHooks is the instrumentable form of the collector.
// onPath runs for every normalized root or include target before authorization,
// stat, or read. onError receives the path responsible for a collection
// failure. Security-sensitive callers use these hooks to retain lexical
// provenance even when a path is missing or malformed and therefore never
// becomes part of the successfully collected file list.
func collectXCConfigFilesWithHooks(
	root string,
	read func(string) ([]byte, error),
	authorize func(string) error,
	onPath func(string),
	onError func(string, error),
) ([]string, error) {
	return collectXCConfigFilesWithHooksAndIdentityAndOptionalMissing(root, read, authorize, onPath, onError, nil, nil)
}

// collectXCConfigFilesWithHooksAndIdentity is the signing-specific collector
// extension for filesystems whose path spelling semantics can vary by
// directory. The identity callback runs only after authorization and lets a
// caller distinguish two case-variant paths that are distinct files without
// weakening the no-read-before-authorization contract.
func collectXCConfigFilesWithHooksAndIdentity(
	root string,
	read func(string) ([]byte, error),
	identify func(string) (os.FileInfo, error),
) ([]string, error) {
	return collectXCConfigFilesWithHooksAndIdentityAndOptionalMissing(root, read, nil, nil, nil, identify, nil)
}

// collectXCConfigFilesWithHooksAndIdentityAndOptionalMissing is the
// instrumentable collector used by signing plan generation. onOptionalMissing
// receives each lexically resolved optional include whose target is absent.
// The callback runs after the authorization check and before the missing target
// is ignored, so callers can persist an absence assertion without granting
// access to an untrusted path.
func collectXCConfigFilesWithHooksAndIdentityAndOptionalMissing(
	root string,
	read func(string) ([]byte, error),
	authorize func(string) error,
	onPath func(string),
	onError func(string, error),
	identify func(string) (os.FileInfo, error),
	onOptionalMissing func(string),
) ([]string, error) {
	return collectXCConfigFilesWithHooksAndIdentityAndOptionalMissingLimit(root, read, authorize, onPath, onError, identify, onOptionalMissing, 0, nil)
}

// xcconfigSourceBudget tracks successfully collected source identities across
// multiple configuration roots. A signing plan may visit the same root once
// per target/configuration, so the budget must count each source only once at
// plan scope while still preserving separate traversals for hard links whose
// relative includes can resolve differently.
type xcconfigSourceBudget struct {
	entries      []xcconfigSourceBudgetEntry
	byPath       map[string][]int
	byFoldedPath map[string][]int
	roots        map[string]xcconfigSourceBudgetRoot
}

type xcconfigSourceBudgetEntry struct {
	path string
	info os.FileInfo
}

// xcconfigSourceBudgetRoot is a completed collection that can be replayed for
// another configuration referring to the same lexical root. Keep the path
// events so signing-plan hooks retain their per-configuration observations,
// while avoiding another stat/read/parse traversal of the source graph.
type xcconfigSourceBudgetRoot struct {
	paths           []string
	pathEvents      []string
	optionalMissing []string
	maxFiles        int
}

func (b *xcconfigSourceBudget) root(path string, maxFiles int) (xcconfigSourceBudgetRoot, bool) {
	if b == nil || b.roots == nil {
		return xcconfigSourceBudgetRoot{}, false
	}
	root, ok := b.roots[normalizeSigningLexicalPath(path)]
	return root, ok && root.maxFiles == maxFiles
}

func (b *xcconfigSourceBudget) cacheRoot(path string, root xcconfigSourceBudgetRoot) {
	if b == nil {
		return
	}
	if b.roots == nil {
		b.roots = make(map[string]xcconfigSourceBudgetRoot)
	}
	root.paths = append([]string(nil), root.paths...)
	root.pathEvents = append([]string(nil), root.pathEvents...)
	root.optionalMissing = append([]string(nil), root.optionalMissing...)
	b.roots[normalizeSigningLexicalPath(path)] = root
}

func (b *xcconfigSourceBudget) contains(path string, info os.FileInfo) bool {
	if b == nil {
		return false
	}
	path = normalizeSigningLexicalPath(path)
	containsEntry := func(indexes []int) bool {
		for _, index := range indexes {
			entry := b.entries[index]
			if entry.path == path {
				if info == nil || entry.info == nil {
					// A collector without identity support can still establish
					// duplicate lexical sources. Signing traversal normally has an
					// identity, so two replaced files at one path remain distinct.
					return true
				}
				if os.SameFile(info, entry.info) {
					return true
				}
				continue
			}
			if info != nil && entry.info != nil && signingPathCaseEquivalent(entry.path, path) && os.SameFile(info, entry.info) {
				return true
			}
		}
		return false
	}
	if b.byPath != nil {
		if containsEntry(b.byPath[path]) {
			return true
		}
	} else {
		for index := range b.entries {
			if containsEntry([]int{index}) {
				return true
			}
		}
	}
	foldedPath := strings.ToLower(path)
	if b.byFoldedPath != nil {
		return containsEntry(b.byFoldedPath[foldedPath])
	}
	for index, entry := range b.entries {
		if strings.ToLower(entry.path) == foldedPath && containsEntry([]int{index}) {
			return true
		}
	}
	return false
}

func (b *xcconfigSourceBudget) count() int {
	if b == nil {
		return 0
	}
	return len(b.entries)
}

func (b *xcconfigSourceBudget) add(path string, info os.FileInfo) bool {
	if b == nil || b.contains(path, info) {
		return false
	}
	if b.byPath == nil {
		b.byPath = make(map[string][]int)
	}
	if b.byFoldedPath == nil {
		b.byFoldedPath = make(map[string][]int)
	}
	path = normalizeSigningLexicalPath(path)
	index := len(b.entries)
	b.entries = append(b.entries, xcconfigSourceBudgetEntry{
		path: path,
		info: info,
	})
	b.byPath[path] = append(b.byPath[path], index)
	b.byFoldedPath[strings.ToLower(path)] = append(b.byFoldedPath[strings.ToLower(path)], index)
	return true
}

// collectXCConfigFilesWithHooksAndIdentityAndOptionalMissingLimit is the
// bounded form used by signing-plan generation. optionalProbe is consulted
// only for an optional include encountered after maxFiles sources have already
// been collected. It must perform a no-follow existence check after the caller
// has authorized the path; os.ErrNotExist means the optional include remains
// absent and does not consume the budget, while any other result is treated as
// a present or indeterminate source and fails with a typed limit error.
func collectXCConfigFilesWithHooksAndIdentityAndOptionalMissingLimit(
	root string,
	read func(string) ([]byte, error),
	authorize func(string) error,
	onPath func(string),
	onError func(string, error),
	identify func(string) (os.FileInfo, error),
	onOptionalMissing func(string),
	maxFiles int,
	optionalProbe func(string) (os.FileInfo, error),
) ([]string, error) {
	return collectXCConfigFilesWithHooksAndIdentityAndOptionalMissingLimitWithBudget(
		root, read, authorize, onPath, onError, identify, onOptionalMissing, maxFiles, optionalProbe, nil,
	)
}

// collectXCConfigFilesWithHooksAndIdentityAndOptionalMissingLimitWithBudget is
// the bounded collector with an optional plan-wide source budget. A nil
// budget retains the historical per-collection bound used by callers outside
// signing-plan generation.
func collectXCConfigFilesWithHooksAndIdentityAndOptionalMissingLimitWithBudget(
	root string,
	read func(string) ([]byte, error),
	authorize func(string) error,
	onPath func(string),
	onError func(string, error),
	identify func(string) (os.FileInfo, error),
	onOptionalMissing func(string),
	maxFiles int,
	optionalProbe func(string) (os.FileInfo, error),
	budget *xcconfigSourceBudget,
) ([]string, error) {
	if budget != nil {
		if cached, ok := budget.root(root, maxFiles); ok {
			for _, path := range cached.pathEvents {
				if onPath != nil {
					onPath(path)
				}
				if authorize != nil {
					if err := authorize(path); err != nil {
						if onError != nil {
							onError(path, err)
						}
						return nil, err
					}
				}
			}
			for _, path := range cached.optionalMissing {
				if optionalProbe == nil {
					if onError != nil {
						onError(path, os.ErrNotExist)
					}
					if onOptionalMissing != nil {
						onOptionalMissing(path)
					}
					continue
				}
				_, probeErr := optionalProbe(path)
				if errors.Is(probeErr, os.ErrNotExist) {
					if onError != nil {
						onError(path, probeErr)
					}
					if onOptionalMissing != nil {
						onOptionalMissing(path)
					}
					continue
				}
				err := newXCConfigSourceGraphLimitError(path, maxFiles, probeErr)
				if onError != nil {
					onError(path, err)
				}
				return nil, err
			}
			return append([]string(nil), cached.paths...), nil
		}
	}
	seen := make(map[string]bool)
	type collectedIdentity struct {
		// key is the collected source's traversal key, which is its normalized
		// spelling whenever an identity callback is in use. Keeping it lets the
		// scans below compare normalized paths without normalizing again.
		key  string
		info os.FileInfo
	}
	var collected []collectedIdentity
	var paths []string
	var pathEvents []string
	var optionalMissing []string
	traversalKey := func(path string) string {
		// With an identity callback, preserve the exact spelling. Identity
		// checks below can safely coalesce an alias only after the platform's
		// directory case semantics prove that the spellings name one path. A
		// generic collector has no such proof and retains its historical
		// platform lexical key for cycle protection.
		if identify != nil {
			return normalizeSigningLexicalPath(path)
		}
		return signingLexicalPathKey(path)
	}
	var visit func(string, map[string][]os.FileInfo, bool) (error, bool)
	visit = func(path string, stack map[string][]os.FileInfo, optional bool) (error, bool) {
		path = filepath.Clean(path)
		pathKey := traversalKey(path)
		if budget != nil {
			pathEvents = append(pathEvents, path)
		}
		if onPath != nil {
			onPath(path)
		}
		if authorize != nil {
			if err := authorize(path); err != nil {
				if onError != nil {
					onError(path, err)
				}
				return err, false
			}
		}
		var identity os.FileInfo
		if identify != nil {
			var err error
			identity, err = identify(path)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				if onError != nil {
					onError(path, err)
				}
				return err, false
			}
		}
		sameIdentity := func(infos []os.FileInfo) bool {
			if identity == nil {
				return false
			}
			for _, info := range infos {
				if info != nil && os.SameFile(identity, info) {
					return true
				}
			}
			return false
		}
		// Case variants can refer to one inode on a case-insensitive volume.
		// Coalesce only spellings that differ by case: general hard links may
		// live in different directories, where their relative includes resolve
		// against different bases and must still be traversed independently.
		if identity != nil {
			for _, entry := range collected {
				if signingPathCaseEquivalentNormalized(entry.key, pathKey) && entry.info != nil && os.SameFile(identity, entry.info) {
					return nil, false
				}
			}
		}
		if stackInfos, ok := stack[pathKey]; ok {
			if identify == nil || sameIdentity(stackInfos) {
				return nil, false
			}
		}
		if seen[pathKey] {
			if identify == nil {
				return nil, false
			}
			for _, entry := range collected {
				if entry.key == pathKey && entry.info != nil && os.SameFile(identity, entry.info) {
					return nil, false
				}
			}
		}
		newBudgetSource := budget == nil || !budget.contains(path, identity)
		collectedCount := len(paths)
		if budget != nil {
			collectedCount = budget.count()
		}
		if maxFiles > 0 && newBudgetSource && collectedCount >= maxFiles {
			if optional && optionalProbe != nil {
				_, probeErr := optionalProbe(path)
				if errors.Is(probeErr, os.ErrNotExist) {
					if onError != nil {
						onError(path, probeErr)
					}
					return probeErr, true
				}
				// A nil probe error, a non-nil file, or any other probe error
				// proves that this is not a safely absent optional include. The
				// source budget must win before the content reader is reached.
				err := newXCConfigSourceGraphLimitError(path, maxFiles, probeErr)
				if onError != nil {
					onError(path, err)
				}
				return err, false
			}
			err := newXCConfigSourceGraphLimitError(path, maxFiles, nil)
			if onError != nil {
				onError(path, err)
			}
			return err, false
		}
		data, err := read(path)
		if err != nil {
			if onError != nil {
				onError(path, err)
			}
			return err, errors.Is(err, os.ErrNotExist)
		}
		document, err := parseXCConfig(data)
		if err != nil {
			if onError != nil {
				onError(path, err)
			}
			return fmt.Errorf("parse %s: %w", path, err), false
		}
		if identify != nil && identity == nil {
			identity, err = identify(path)
			if err != nil {
				if onError != nil {
					onError(path, err)
				}
				return err, false
			}
		}
		seen[pathKey] = true
		paths = append(paths, path)
		if identity != nil {
			collected = append(collected, collectedIdentity{key: pathKey, info: identity})
		}
		if budget != nil {
			budget.add(path, identity)
		}
		// The stack is scoped to this depth-first visit. Mutate it in place and
		// restore the entry when this node returns so a long include chain does
		// not copy every ancestor map at every level.
		previousStackInfos, hadStackEntry := stack[pathKey]
		stack[pathKey] = append(previousStackInfos, identity)
		defer func() {
			if hadStackEntry {
				stack[pathKey] = previousStackInfos
				return
			}
			delete(stack, pathKey)
		}()
		var includeErrors []error
		for _, include := range document.includes {
			includePath, err := resolveXCConfigInclude(path, include)
			if err != nil {
				if onError != nil {
					onError(path, err)
				}
				includeErrors = append(includeErrors, err)
				continue
			}
			// Let the same authorization-aware reader perform existence and type
			// checks. In particular, never stat an include before the authorization
			// hook has accepted its lexical path. Optional missing includes are the
			// one intentional not-exist case and are ignored after that check.
			childErr, missingTarget := visit(includePath, stack, include.optional)
			if childErr != nil {
				if isXCConfigSourceGraphLimitError(childErr) {
					return childErr, false
				}
				if include.optional && missingTarget {
					if budget != nil {
						optionalMissing = append(optionalMissing, includePath)
					}
					if onOptionalMissing != nil {
						onOptionalMissing(includePath)
					}
					continue
				}
				includeErrors = append(includeErrors, childErr)
			}
		}
		if len(includeErrors) == 1 {
			return includeErrors[0], false
		}
		if len(includeErrors) > 1 {
			return errors.Join(includeErrors...), false
		}
		return nil, false
	}
	if err, _ := visit(root, make(map[string][]os.FileInfo), false); err != nil {
		return nil, err
	}
	if budget != nil {
		budget.cacheRoot(root, xcconfigSourceBudgetRoot{
			paths:           paths,
			pathEvents:      pathEvents,
			optionalMissing: optionalMissing,
			maxFiles:        maxFiles,
		})
	}
	return paths, nil
}

// signingPathCaseEquivalent reports whether two path spellings may be the same
// path solely because their containing directory is case-insensitive.
// Equal-folding a path is not enough: a case-sensitive directory can contain
// two hard-linked files whose path operations must remain distinct. Unknown
// filesystem metadata therefore keeps both spellings rather than coalescing.
func signingPathCaseEquivalent(left, right string) bool {
	return signingPathCaseEquivalentNormalized(normalizeSigningLexicalPath(left), normalizeSigningLexicalPath(right))
}

// signingPathCaseEquivalentNormalized answers the same question as
// signingPathCaseEquivalent for two already normalized paths. An include
// traversal compares one path against every ancestor or already collected
// source, so normalizing each candidate once at its own visit keeps those
// scans from re-normalizing the same paths on every comparison.
func signingPathCaseEquivalentNormalized(left, right string) bool {
	if left == right {
		return true
	}
	if !strings.EqualFold(left, right) {
		return false
	}
	leftInsensitive, leftKnown := signingCaseInsensitiveVolumeFn(filepath.Dir(left))
	rightInsensitive, rightKnown := signingCaseInsensitiveVolumeFn(filepath.Dir(right))
	return leftKnown && rightKnown && leftInsensitive && rightInsensitive
}

func resolveXCConfigSetting(root, setting string) (xcconfigResolvedValue, error) {
	return resolveXCConfigSettingWithBase(root, setting, xcconfigResolvedValue{})
}

func resolveXCConfigSettingWithBase(root, setting string, base xcconfigResolvedValue) (xcconfigResolvedValue, error) {
	var identify func(string) (os.FileInfo, error)
	if xcconfigUsesIdentityTraversal() {
		// Identity-aware traversal coalesces case-variant aliases on
		// case-insensitive volumes, including Linux vfat/exfat/ntfs mounts.
		// os.Stat supplies the identity; case-semantics checks keep genuinely
		// distinct files separate.
		identify = os.Stat
	}
	return resolveXCConfigSettingWithBaseReaderAndIdentity(root, setting, base, os.ReadFile, os.Stat, identify)
}

func resolveXCConfigSettingWithBaseReader(
	root, setting string,
	base xcconfigResolvedValue,
	read func(string) ([]byte, error),
	stat func(string) (os.FileInfo, error),
) (xcconfigResolvedValue, error) {
	return resolveXCConfigSettingWithBaseReaderAndIdentity(root, setting, base, read, stat, nil)
}

func resolveXCConfigSettingWithBaseReaderAndIdentity(
	root, setting string,
	base xcconfigResolvedValue,
	read func(string) ([]byte, error),
	stat func(string) (os.FileInfo, error),
	identify func(string) (os.FileInfo, error),
) (xcconfigResolvedValue, error) {
	return resolveXCConfigSettingWithBaseReaderAndIdentityAndLookup(root, setting, base, read, stat, identify, nil)
}

func resolveXCConfigSettingWithBaseReaderAndIdentityAndLookup(
	root, setting string,
	base xcconfigResolvedValue,
	read func(string) ([]byte, error),
	stat func(string) (os.FileInfo, error),
	identify func(string) (os.FileInfo, error),
	lookup func(string) (string, bool),
) (xcconfigResolvedValue, error) {
	resolved, conditional, err := resolveXCConfigSettingStateWithReaderAndIdentity(
		root, setting, base, read, stat, identify, nil, lookup,
	)
	if err != nil {
		return xcconfigResolvedValue{}, err
	}
	if !resolved.exact && conditional {
		return xcconfigResolvedValue{}, fmt.Errorf(
			"%s is defined only by conditional xcconfig assignments; SDK-aware resolution requires Xcode",
			setting,
		)
	}
	if resolved.exact {
		for _, conditionalValue := range resolved.conditionals {
			if conditionalValue.operator != "=" || strings.TrimSpace(conditionalValue.value) != strings.TrimSpace(resolved.value) {
				return xcconfigResolvedValue{}, fmt.Errorf(
					"%s has differing conditional xcconfig assignment %s %s %q in %s (unconditional value %q); narrow the scope or use Xcode-aware resolution",
					setting,
					conditionalValue.key,
					conditionalValue.operator,
					conditionalValue.value,
					conditionalValue.path,
					resolved.value,
				)
			}
		}
	}
	if resolved.missingInherited {
		return xcconfigResolvedValue{}, fmt.Errorf("%s uses $(inherited) without a lower-layer value", setting)
	}
	return resolved, nil
}

// expandXCConfigLookupReferences expands only references supplied by lookup.
// Unsupported or unresolved references stay intact so divergence checks remain
// conservative rather than guessing a build-context value.
func expandXCConfigLookupReferences(value string, lookup func(string) (string, bool)) string {
	if lookup == nil {
		return value
	}
	for iteration := 0; iteration < 32; iteration++ {
		match := signingReferencePattern.FindStringSubmatchIndex(value)
		if match == nil || match[4] >= 0 || match[8] >= 0 {
			return value
		}
		nameStart, nameEnd := match[2], match[3]
		if nameStart < 0 {
			nameStart, nameEnd = match[6], match[7]
		}
		replacement, ok := lookup(value[nameStart:nameEnd])
		if !ok {
			return value
		}
		value = value[:match[0]] + replacement + value[match[1]:]
	}
	return value
}

// xcconfigAssignmentObserver receives each matching assignment in the same
// include/event order used by the resolver, including assignments that the
// resolver later skips because a lower or earlier value wins. Security-
// sensitive callers use this to retain provenance without implementing a
// second include walker or authorization path.
type xcconfigAssignmentObserver func(path string, assignment xcconfigAssignment)

// resolveXCConfigSettingStateWithReaderAndIdentity exposes the raw traversal
// state to narrow provenance consumers. Unlike the public resolution wrapper,
// it does not convert conditional-only or divergent assignments into a
// resolution error; operational read/parse/include failures still propagate.
func resolveXCConfigSettingStateWithReaderAndIdentity(
	root, setting string,
	base xcconfigResolvedValue,
	read func(string) ([]byte, error),
	stat func(string) (os.FileInfo, error),
	identify func(string) (os.FileInfo, error),
	observe xcconfigAssignmentObserver,
	lookup func(string) (string, bool),
) (xcconfigResolvedValue, bool, error) {
	return resolveXCConfigSettingRecursiveWithReaderAndIdentity(
		filepath.Clean(root), setting, make(map[string]bool), new([]xcconfigResolutionPath), base, read, stat, identify, observe, lookup,
	)
}

type xcconfigResolutionPath struct {
	// key is the ancestor's normalized path. The scan over the ancestors
	// compares normalized spellings, so each ancestor is normalized once when
	// it is pushed rather than once per comparison.
	key  string
	info os.FileInfo
}

func resolveXCConfigSettingRecursiveWithReaderAndIdentity(
	path string,
	setting string,
	stack map[string]bool,
	stackPaths *[]xcconfigResolutionPath,
	resolved xcconfigResolvedValue,
	read func(string) ([]byte, error),
	stat func(string) (os.FileInfo, error),
	identify func(string) (os.FileInfo, error),
	observe xcconfigAssignmentObserver,
	lookup func(string) (string, bool),
) (xcconfigResolvedValue, bool, error) {
	path = filepath.Clean(path)
	pathKey := signingLexicalPathKey(path)
	var identity os.FileInfo
	if identify != nil {
		var err error
		identity, err = identify(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return xcconfigResolvedValue{}, false, err
		}
		pathKey = normalizeSigningLexicalPath(path)
		for _, entry := range *stackPaths {
			if identity != nil && entry.info != nil && signingPathCaseEquivalentNormalized(entry.key, pathKey) && os.SameFile(identity, entry.info) {
				return resolved, false, nil
			}
		}
	}
	if stack[pathKey] {
		return resolved, false, nil
	}
	data, err := read(path)
	if err != nil {
		return xcconfigResolvedValue{}, false, err
	}
	document, err := parseXCConfig(data)
	if err != nil {
		return xcconfigResolvedValue{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	// This file is an ancestor for the include recursion below and stops being
	// one as soon as this call returns. Cloning the ancestor set and the
	// ancestor path list per level made a deep include chain quadratic in
	// allocations while detecting exactly the same cycles as a push/pop stack.
	// The pathKey check above guarantees it is not already on the stack, so the
	// entry this call pushed is the one it removes.
	stack[pathKey] = true
	pushedStackPath := identify != nil && identity != nil
	if pushedStackPath {
		*stackPaths = append(*stackPaths, xcconfigResolutionPath{key: pathKey, info: identity})
	}
	defer func() {
		delete(stack, pathKey)
		if pushedStackPath {
			*stackPaths = (*stackPaths)[:len(*stackPaths)-1]
		}
	}()

	type event struct {
		line       int
		assignment *xcconfigAssignment
		include    *xcconfigInclude
	}
	var events []event
	for index := range document.assignments {
		assignment := &document.assignments[index]
		events = append(events, event{line: assignment.lineIndex, assignment: assignment})
	}
	for index := range document.includes {
		include := &document.includes[index]
		events = append(events, event{line: include.lineIndex, include: include})
	}
	sort.SliceStable(events, func(left, right int) bool {
		return events[left].line < events[right].line
	})

	for _, item := range events {
		if item.include != nil {
			includePath, err := resolveXCConfigInclude(path, *item.include)
			if err != nil {
				return xcconfigResolvedValue{}, false, err
			}
			if _, err := stat(includePath); err != nil {
				if item.include.optional && os.IsNotExist(err) {
					continue
				}
				return xcconfigResolvedValue{}, false, fmt.Errorf("read xcconfig include %s: %w", includePath, err)
			}
			included, _, err := resolveXCConfigSettingRecursiveWithReaderAndIdentity(includePath, setting, stack, stackPaths, resolved, read, stat, identify, observe, lookup)
			if err != nil {
				return xcconfigResolvedValue{}, false, err
			}
			resolved = included
			continue
		}

		assignment := item.assignment
		if assignment.baseKey != setting {
			continue
		}
		if observe != nil {
			observe(path, *assignment)
		}
		if !resolved.found && lookup != nil {
			if implicit, ok := lookup(setting); ok {
				// An implicit value is a lower-layer value, not a replacement
				// for the conditional assignments already seen in this file.
				// Keep explicit conditionals so the caller can reject a
				// divergent SDK-specific value, while a conditional default
				// remains shadowed by the implicit value just like ?= would be
				// by any other lower-layer assignment.
				conditionals := make([]xcconfigConditionalValue, 0, len(resolved.conditionals))
				for _, conditional := range resolved.conditionals {
					if conditional.operator != "?=" {
						conditionals = append(conditionals, conditional)
					}
				}
				resolved = xcconfigResolvedValue{
					value:        implicit,
					path:         "<implicit>",
					found:        true,
					exact:        true,
					conditionals: conditionals,
				}
			}
		}
		if assignment.key != setting {
			selector := signingXCConfigSelectorIdentity(assignment.key)
			inheritedValue := resolved.value
			for index := len(resolved.conditionals) - 1; index >= 0; index-- {
				if signingXCConfigSelectorIdentity(resolved.conditionals[index].key) == selector {
					inheritedValue = resolved.conditionals[index].value
					break
				}
			}
			if assignment.operator == "?=" && resolved.found {
				continue
			}
			if assignment.operator == "=" {
				filtered := make([]xcconfigConditionalValue, 0, len(resolved.conditionals))
				for _, existing := range resolved.conditionals {
					if signingXCConfigSelectorIdentity(existing.key) == selector {
						continue
					}
					filtered = append(filtered, existing)
				}
				resolved.conditionals = filtered
			}
			conditionalValue := assignment.value
			if resolved.found {
				if strings.Contains(conditionalValue, "$(inherited)") || strings.Contains(conditionalValue, "${inherited}") {
					conditionalValue = strings.ReplaceAll(conditionalValue, "$(inherited)", inheritedValue)
					conditionalValue = strings.ReplaceAll(conditionalValue, "${inherited}", inheritedValue)
				}
				conditionalValue = expandXCConfigLookupReferences(conditionalValue, lookup)
			}
			resolved.conditionals = append(resolved.conditionals, xcconfigConditionalValue{
				key:      assignment.key,
				value:    conditionalValue,
				operator: assignment.operator,
				path:     path,
			})
			continue
		}
		value := assignment.value
		hadLowerValue := resolved.found
		hasInherited := strings.Contains(value, "$(inherited)") || strings.Contains(value, "${inherited}")
		value = strings.ReplaceAll(value, "$(inherited)", resolved.value)
		value = strings.ReplaceAll(value, "${inherited}", resolved.value)
		switch assignment.operator {
		case "?=":
			if resolved.found {
				continue
			}
		case "+=":
			if !hasInherited {
				value = strings.TrimSpace(strings.TrimSpace(resolved.value) + " " + strings.TrimSpace(value))
			}
		}
		conditionals := append([]xcconfigConditionalValue(nil), resolved.conditionals...)
		if assignment.operator == "=" && !hasInherited {
			// A later unconditional replacement shadows earlier conditional
			// defaults, but an explicit conditional assignment remains relevant
			// in its build context and must still be reconciled below.
			conditionals = conditionals[:0]
			for _, conditional := range resolved.conditionals {
				if conditional.operator != "?=" {
					conditionals = append(conditionals, conditional)
				}
			}
		}
		resolved = xcconfigResolvedValue{
			value:            strings.TrimSpace(value),
			path:             path,
			found:            true,
			exact:            true,
			missingInherited: hasInherited && !hadLowerValue,
			conditionals:     conditionals,
		}
	}
	return resolved, len(resolved.conditionals) > 0, nil
}

func editXCConfig(data []byte, setting, value string) ([]byte, []string, bool, error) {
	document, err := parseXCConfig(data)
	if err != nil {
		return nil, nil, false, err
	}
	assignmentsByLine := make(map[int]xcconfigAssignment)
	var oldValues []string
	for _, assignment := range document.assignments {
		if assignment.baseKey != setting {
			continue
		}
		assignmentsByLine[assignment.lineIndex] = assignment
		oldValues = append(oldValues, assignment.value)
	}
	if len(assignmentsByLine) == 0 {
		return data, nil, false, nil
	}

	changed := false
	for index, assignment := range assignmentsByLine {
		line := document.lines[index]
		if assignment.value == value && assignment.operator == "=" {
			continue
		}
		quotedValue := quoteXCConfigValue(value, assignment.quote)
		document.lines[index] = line[:assignment.operatorStart] + "=" +
			line[assignment.operatorEnd:assignment.valueStart] + quotedValue + line[assignment.valueEnd:]
		changed = true
	}
	return []byte(strings.Join(document.lines, "")), oldValues, changed, nil
}

func quoteXCConfigValue(value, quote string) string {
	if quote == "" {
		return value
	}
	var encoded strings.Builder
	encoded.Grow(len(value) + len(quote)*2)
	encoded.WriteString(quote)
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character == '\\' || character == quote[0] {
			encoded.WriteByte('\\')
		}
		encoded.WriteByte(character)
	}
	encoded.WriteString(quote)
	return encoded.String()
}
