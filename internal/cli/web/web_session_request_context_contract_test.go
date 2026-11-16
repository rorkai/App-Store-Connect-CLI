package web

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

var webSessionResolveNames = map[string]struct{}{
	"resolveWebSessionForCommand":                {},
	"resolveWebSessionForCommandWith":            {},
	"callResolveSessionForProviderSelection":     {},
	"callResolveSessionForProviderSelectionWith": {},
	"resolveWebSession":                          {},
	"callResolveAppCreateSessionFn":              {},
	"resolveWebComplianceClient":                 {},
}

// TestWebCommandsDoNotStartRequestTimeoutBeforeAuthentication is the drift
// guard for #2333. New web commands must not recreate the expired-context
// shape that a per-command patch would miss. Independent bounded preflights
// that do not flow into session resolution are allowed.
func TestWebCommandsDoNotStartRequestTimeoutBeforeAuthentication(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate web request-context contract test")
	}
	webDir := filepath.Dir(currentFile)
	fset := token.NewFileSet()
	var violations []string

	err := filepath.WalkDir(webDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		relative, relErr := filepath.Rel(webDir, path)
		if relErr != nil {
			relative = path
		}
		if strings.Contains(string(src), "resolveWebSessionForCommand(requestCtx") {
			violations = append(violations, fmt.Sprintf("%s: resolveWebSessionForCommand(requestCtx reuses the pre-auth timeout", relative))
		}

		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch fn := node.(type) {
			case *ast.FuncDecl:
				violations = append(violations, requestTimeoutBeforeAuthViolations(fset, relative, fn.Name.Name, fn.Body)...)
			case *ast.FuncLit:
				violations = append(violations, requestTimeoutBeforeAuthViolations(fset, relative, "func literal", fn.Body)...)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan web request-context contract: %v", err)
	}
	if len(violations) == 0 {
		return
	}

	sort.Strings(violations)
	t.Fatalf(
		"web commands must resolve the session before starting the request timeout (%d violation(s)):\n%s",
		len(violations),
		strings.Join(violations, "\n"),
	)
}

func requestTimeoutBeforeAuthViolations(fset *token.FileSet, relative, funcName string, body *ast.BlockStmt) []string {
	if body == nil {
		return nil
	}

	timeoutNames := map[string]token.Pos{}
	var violations []string
	ast.Inspect(body, func(node ast.Node) bool {
		if _, isLit := node.(*ast.FuncLit); isLit {
			return false
		}
		switch stmt := node.(type) {
		case *ast.AssignStmt:
			recordTimeoutAssignments(stmt, timeoutNames)
		case *ast.CallExpr:
			name := identName(stmt.Fun)
			if _, isResolve := webSessionResolveNames[name]; !isResolve || len(stmt.Args) == 0 {
				return true
			}
			switch arg := stmt.Args[0].(type) {
			case *ast.Ident:
				if assignedAt, timed := timeoutNames[arg.Name]; timed && assignedAt < stmt.Pos() {
					position := fset.Position(stmt.Pos())
					violations = append(violations, fmt.Sprintf("%s:%d: %s passes a ContextWithTimeout result to %s", relative, position.Line, funcName, name))
				}
				if (name == "resolveWebSessionForCommand" || name == "resolveWebSessionForCommandWith") && arg.Name == "requestCtx" {
					position := fset.Position(stmt.Pos())
					violations = append(violations, fmt.Sprintf("%s:%d: %s calls resolveWebSessionForCommand(requestCtx", relative, position.Line, funcName))
				}
			case *ast.CallExpr:
				if isRequestTimeoutStartCall(arg.Fun) {
					position := fset.Position(stmt.Pos())
					violations = append(violations, fmt.Sprintf("%s:%d: %s passes a request-timeout context directly to %s", relative, position.Line, funcName, name))
				}
			}
		}
		return true
	})
	return uniqueStrings(violations)
}

func recordTimeoutAssignments(stmt *ast.AssignStmt, timeoutNames map[string]token.Pos) {
	if len(stmt.Rhs) != 1 || !isRequestTimeoutStartCallExpr(stmt.Rhs[0]) || len(stmt.Lhs) == 0 {
		return
	}
	ident, ok := stmt.Lhs[0].(*ast.Ident)
	if !ok || ident.Name == "" || ident.Name == "_" {
		return
	}
	timeoutNames[ident.Name] = stmt.Pos()
}

func isRequestTimeoutStartCallExpr(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	return ok && isRequestTimeoutStartCall(call.Fun)
}

func isRequestTimeoutStartCall(fun ast.Expr) bool {
	if name := identName(fun); name == "newWebRequestContext" || name == "newWebRequestContextWith" {
		return true
	}
	selector, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	switch pkg.Name {
	case "shared":
		return selector.Sel.Name == "ContextWithTimeout" || selector.Sel.Name == "ContextWithUploadTimeout"
	case "d":
		return selector.Sel.Name == "Timeout" || selector.Sel.Name == "UploadTimeout"
	}
	return false
}

func identName(fun ast.Expr) string {
	ident, ok := fun.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
