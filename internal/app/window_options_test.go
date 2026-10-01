package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"runtime"
	"testing"
)

func TestResidentLifecycleUsesApplicationVisibility(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "app.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	calls := make(map[string]int)
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok || pkg.Name != "runtime" {
			return true
		}
		calls[selector.Sel.Name]++
		return true
	})
	for _, name := range []string{"WindowHide", "WindowShow"} {
		if calls[name] != 0 {
			t.Errorf("resident lifecycle must not mix application hiding with runtime.%s: %d calls", name, calls[name])
		}
	}
	if calls["Hide"] == 0 || calls["Show"] == 0 {
		t.Fatal("resident lifecycle must pair runtime.Hide with runtime.Show")
	}
}

func TestPlatformWindowChrome(t *testing.T) {
	if got, want := platformFrameless(), runtime.GOOS == "windows"; got != want {
		t.Fatalf("frameless = %v, want %v on %s", got, want, runtime.GOOS)
	}
	if opts := platformWindowsOptions(); runtime.GOOS == "windows" {
		if opts == nil || opts.DisableFramelessWindowDecorations {
			t.Fatal("Windows custom title bar must retain native resize decorations")
		}
	} else if opts != nil {
		t.Fatal("Windows window options must not affect other platforms")
	}
}
