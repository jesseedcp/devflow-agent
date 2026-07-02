package codemap

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
)

func IndexGoFacts(root string) (Index, error) {
	root = filepath.Clean(root)
	idx := Index{Version: "v0.4", Root: root}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".devflow", "dist", "node_modules", ".worktrees":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		facts, err := indexGoFile(root, path)
		if err != nil {
			return err
		}
		idx.Facts = append(idx.Facts, facts...)
		return nil
	})
	return idx, err
}

func indexGoFile(root, path string) ([]CodeFact, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	var facts []CodeFact
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			pos := fileSet.Position(decl.Pos())
			kind := "func"
			receiver := ""
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				kind = "method"
				receiver = exprString(decl.Recv.List[0].Type)
			}
			if strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(decl.Name.Name, "Test") {
				kind = "test"
			}
			facts = append(facts, CodeFact{
				Kind:      kind,
				Package:   file.Name.Name,
				File:      rel,
				Name:      decl.Name.Name,
				Receiver:  receiver,
				Line:      pos.Line,
				Signature: funcSignature(decl),
				Text:      commentText(decl.Doc),
			})
			facts = append(facts, routeFacts(file.Name.Name, rel, fileSet, decl.Body)...)
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				pos := fileSet.Position(typeSpec.Pos())
				facts = append(facts, CodeFact{Kind: "type", Package: file.Name.Name, File: rel, Name: typeSpec.Name.Name, Line: pos.Line, Text: commentText(decl.Doc)})
			}
		}
	}
	return facts, nil
}

func routeFacts(pkg, file string, fileSet *token.FileSet, body *ast.BlockStmt) []CodeFact {
	if body == nil {
		return nil
	}
	var facts []CodeFact
	ast.Inspect(body, func(node ast.Node) bool {
		lit, ok := node.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		if strings.HasPrefix(value, "/") || strings.Contains(value, "http://") || strings.Contains(value, "https://") {
			pos := fileSet.Position(lit.Pos())
			facts = append(facts, CodeFact{Kind: "route", Package: pkg, File: file, Name: value, Line: pos.Line, Text: value})
		}
		return true
	})
	return facts
}

func exprString(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.StarExpr:
		return "*" + exprString(expr.X)
	default:
		return ""
	}
}

func funcSignature(fn *ast.FuncDecl) string {
	var b strings.Builder
	b.WriteString("func ")
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		b.WriteString("(")
		b.WriteString(exprString(fn.Recv.List[0].Type))
		b.WriteString(") ")
	}
	b.WriteString(fn.Name.Name)
	return b.String()
}

func commentText(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	return strings.Join(strings.Fields(group.Text()), " ")
}
