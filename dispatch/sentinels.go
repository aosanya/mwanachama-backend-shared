package dispatch

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
)

func UnmappedSentinels(s *Spec, sentinels map[string]error, dirs ...string) []string {
	var problems []string

	for name := range sentinels {
		if _, ok := s.Errors[name]; !ok {
			problems = append(problems, fmt.Sprintf(
				"%s is supplied to the dispatcher and the spec maps it to no status, so it is redacted to the fallback", name))
		}
	}

	for _, dir := range dirs {
		exported, err := exportedSentinels(dir)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		for _, name := range exported {
			if _, ok := s.Errors[name]; ok {
				continue
			}
			problems = append(problems, fmt.Sprintf(
				"%s is exported by %s and the spec maps it to no status, so it is redacted to the fallback", name, dir))
		}
	}

	sort.Strings(problems)
	return problems
}

func exportedSentinels(dir string) ([]string, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		return nil, fmt.Errorf("dispatch: parse %s: %w", dir, err)
	}

	var out []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, d := range file.Decls {
				decl, ok := d.(*ast.GenDecl)
				if !ok || decl.Tok != token.VAR {
					continue
				}
				for _, s := range decl.Specs {
					value, ok := s.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range value.Names {
						if !name.IsExported() || !strings.HasPrefix(name.Name, "Err") {
							continue
						}
						if i < len(value.Values) && isErrorsNew(value.Values[i]) {
							out = append(out, name.Name)
						}
					}
				}
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

func isErrorsNew(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return pkg.Name == "errors" && sel.Sel.Name == "New"
}
