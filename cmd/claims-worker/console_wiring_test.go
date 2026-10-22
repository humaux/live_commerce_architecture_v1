// Purpose: pin run()'s console bridge wiring (#147, review-148 P2-R2). TestConsoleServerCancelsInflightRequestsWithRunCtx
// exercises newConsoleServer directly, so it stays green if run() passes context.Background() (or any ctx not cancelled at
// shutdown) instead of pollCtx. This test reads the run() call site and fails on that change: the BaseContext must be the
// pollCtx that stopPoll() cancels before joinConsole waits on console.Run's stopAll.
// Depends on: go/ast, go/parser, go/types (source of main.go, not the binary, no database).
// Used by: go test ./cmd/claims-worker (Foundation unit shard).

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

// TestRunWiresPollCtxIntoConsoleServer is a source-level pin: run() opens four database pools before it builds the
// server (main.go:235-266), so no unit test can execute that call site without Postgres. The assertion is deliberately
// narrow — the third argument to newConsoleServer and the stopPoll-before-joinConsole order — so a refactor that keeps
// the shutdown guarantee passes, while wiring any other ctx fails with a concrete message.
func TestRunWiresPollCtxIntoConsoleServer(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}
	var runFn *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "run" {
			runFn = fn
		}
	}
	if runFn == nil {
		t.Fatal("run() not found in main.go")
	}

	var (
		declaredPollCtx               bool
		serverCalls                   int
		serverArg                     ast.Expr
		stopPollLine, joinConsoleLine = -1, -1
	)
	ast.Inspect(runFn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			if node.Tok != token.DEFINE || len(node.Lhs) != 2 || len(node.Rhs) != 1 {
				return true
			}
			ctxName, okCtx := node.Lhs[0].(*ast.Ident)
			stopName, okStop := node.Lhs[1].(*ast.Ident)
			call, okCall := node.Rhs[0].(*ast.CallExpr)
			if !okCtx || !okStop || !okCall || ctxName.Name != "pollCtx" || stopName.Name != "stopPoll" {
				return true
			}
			sel, okSel := call.Fun.(*ast.SelectorExpr)
			if !okSel {
				return true
			}
			if pkg, okPkg := sel.X.(*ast.Ident); okPkg && pkg.Name == "context" && sel.Sel.Name == "WithCancel" {
				declaredPollCtx = true
			}
		case *ast.CallExpr:
			ident, ok := node.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			switch ident.Name {
			case "newConsoleServer":
				if len(node.Args) == 3 {
					serverCalls++
					serverArg = node.Args[2]
				}
			case "stopPoll":
				stopPollLine = fset.Position(node.Pos()).Line
			case "joinConsole":
				joinConsoleLine = fset.Position(node.Pos()).Line
			}
		}
		return true
	})

	if !declaredPollCtx {
		t.Fatal("run() no longer declares pollCtx, stopPoll := context.WithCancel(...); the shutdown ordering below is unpinned")
	}
	if serverCalls != 1 {
		t.Fatalf("run() calls newConsoleServer %d times, want exactly 1", serverCalls)
	}
	if arg, ok := serverArg.(*ast.Ident); !ok || arg.Name != "pollCtx" {
		t.Fatalf("run() passes %s to newConsoleServer; want pollCtx, whose stopPoll() runs before joinConsole waits on "+
			"console.Run's stopAll (context.Background() reopens the #147 shutdown hang)", types.ExprString(serverArg))
	}
	if stopPollLine < 0 || joinConsoleLine < 0 {
		t.Fatalf("run() must call stopPoll() and joinConsole(); found stopPoll at line %d, joinConsole at line %d", stopPollLine, joinConsoleLine)
	}
	if stopPollLine >= joinConsoleLine {
		t.Fatalf("stopPoll() is at main.go:%d but joinConsole at main.go:%d; the ctx handed to the bridge must be cancelled "+
			"before the join waits on stopAll", stopPollLine, joinConsoleLine)
	}
}
