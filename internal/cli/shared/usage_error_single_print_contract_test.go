package shared

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// printingUsageHelpers write their own "Error:" line before returning.
var printingUsageHelpers = map[string]bool{
	"UsageError":     true,
	"UsageErrorf":    true,
	"UsageErrorCtx":  true,
	"UsageErrorfCtx": true,
	"RequiredFlag":   true,
}

func TestUsageErrorCallSitesPrintTheErrorOnce(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate usage error contract test")
	}
	cliDir := filepath.Dir(filepath.Dir(currentFile))
	fset := token.NewFileSet()
	var repeated []string

	err := filepath.WalkDir(cliDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			var statements []ast.Stmt
			switch block := node.(type) {
			case *ast.BlockStmt:
				statements = block.List
			case *ast.CaseClause:
				statements = block.Body
			case *ast.CommClause:
				statements = block.Body
			}
			for index := 1; index < len(statements); index++ {
				if writesErrorLine(statements[index-1]) && returnsPrintingUsageHelper(statements[index]) {
					position := fset.Position(statements[index-1].Pos())
					relative, relErr := filepath.Rel(cliDir, position.Filename)
					if relErr != nil {
						relative = position.Filename
					}
					repeated = append(repeated, fmt.Sprintf("%s:%d", relative, position.Line))
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan usage error call sites: %v", err)
	}
	if len(repeated) > 0 {
		sort.Strings(repeated)
		t.Fatalf("found %d Error: lines followed by a usage helper that prints the error again; return MissingRequiredUsageError or InvalidValueUsageError after the line, or drop the line and let the helper print it:\n%s",
			len(repeated), strings.Join(repeated, "\n"))
	}
}

func writesErrorLine(statement ast.Stmt) bool {
	expression, ok := statement.(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := expression.X.(*ast.CallExpr)
	if !ok || len(call.Args) < 2 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !strings.HasPrefix(selector.Sel.Name, "Fprint") {
		return false
	}
	text := call.Args[1]
	for {
		binary, ok := text.(*ast.BinaryExpr)
		if !ok {
			break
		}
		text = binary.X
	}
	literal, ok := text.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return false
	}
	value, err := strconv.Unquote(literal.Value)
	return err == nil && strings.HasPrefix(value, "Error:")
}

func returnsPrintingUsageHelper(statement ast.Stmt) bool {
	returned, ok := statement.(*ast.ReturnStmt)
	if !ok {
		return false
	}
	for _, result := range returned.Results {
		call, ok := result.(*ast.CallExpr)
		if !ok {
			continue
		}
		switch function := call.Fun.(type) {
		case *ast.Ident:
			if printingUsageHelpers[function.Name] {
				return true
			}
		case *ast.SelectorExpr:
			if printingUsageHelpers[function.Sel.Name] {
				return true
			}
		}
	}
	return false
}
