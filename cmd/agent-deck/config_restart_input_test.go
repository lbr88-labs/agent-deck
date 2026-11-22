package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// An attach/detach command may restart the runtime to reload its configuration.
// A user's draft may already be in the new prompt when that restart returns.
func TestConfigurationRestartDoesNotDriveTerminal(t *testing.T) {
	for filename, targets := range map[string][]string{"skill_cmd.go": {"restartProjectSkillsSession"}, "mcp_cmd.go": {"handleMCPAttach", "handleMCPDetach"}} {
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range targets {
			decl := file.Scope.Lookup(name)
			if decl == nil {
				t.Fatalf("missing command %s", name)
			}
			ast.Inspect(decl.Decl.(*ast.FuncDecl).Body, func(node ast.Node) bool {
				if call, ok := node.(*ast.SelectorExpr); ok {
					method := call.Sel.Name
					if method == "GetTmuxSession" || strings.HasPrefix(method, "SendKeys") || method == "SendEnter" {
						t.Errorf("%s drives a terminal via %s", name, method)
					}
				}
				return true
			})
		}
	}
}
