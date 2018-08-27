package formatx

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"path"
	"runtime"
	"strconv"
	"golang.org/x/tools/go/packages"
	"strings"
	"path/filepath"
)

func SortImportsProcess(fset *token.FileSet, f *ast.File, filename string) error {
	ast.SortImports(fset, f)
	dir := path.Dir(filename)

	for _, decl := range f.Decls {
		d, ok := decl.(*ast.GenDecl)
		if !ok || d.Tok != token.IMPORT || len(d.Specs) == 0 {
			break
		}

		g := &groupSet{}

		for i := range d.Specs {
			g.register(d.Specs[i].(*ast.ImportSpec), dir)
		}

		fileSet, file, err := ParseFile(filename, bytes.Replace(formatNode(fset, f), formatNode(fset, d), g.Bytes(), 1))
		if err != nil {
			return err
		}
		*fset = *fileSet
		*f = *file
	}
	return nil
}

func formatNode(fileSet *token.FileSet, node ast.Node) []byte {
	buf := &bytes.Buffer{}
	if err := format.Node(buf, fileSet, node); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

type groupSet [4][]*dep

type dep struct {
	pkg        *packages.Package
	importSpec *ast.ImportSpec
}

func (group *groupSet) Bytes() []byte {
	buf := bytes.NewBuffer(nil)

	buf.WriteString("import (")

	for _, deps := range group {
		for _, d := range deps {
			buf.WriteRune('\n')

			importSpec := d.importSpec
			if importSpec.Doc != nil {
				for _, c := range importSpec.Doc.List {
					buf.WriteString(c.Text)
					buf.WriteRune('\n')
				}
			}
			if importSpec.Name != nil && importSpec.Name.String() != d.pkg.Name {
				buf.WriteString(importSpec.Name.String())
				buf.WriteRune(' ')
			}
			buf.WriteString(importSpec.Path.Value)
			if importSpec.Comment != nil {
				for _, c := range importSpec.Comment.List {
					buf.WriteString(c.Text)
				}
			}
		}
		buf.WriteRune('\n')
	}

	buf.WriteRune(')')
	return buf.Bytes()
}

var goroot = runtime.GOROOT()

func (group *groupSet) register(importSpec *ast.ImportSpec, dir string) {
	importPath, _ := strconv.Unquote(importSpec.Path.Value)
	pkgs, err := packages.Load(nil, importPath)

	pkg := pkgs[0]

	appendTo := func(i int) {
		group[i] = append(group[i], &dep{
			pkg:        pkg,
			importSpec: importSpec,
		})
	}

	if err != nil {
		appendTo(3)
		return
	}

	importPkgDir := filepath.Dir(pkg.GoFiles[0])

	// std
	if strings.HasPrefix(strings.ToLower(importPkgDir), strings.ToLower(goroot)) {
		appendTo(0)
		return
	}

	// local
	if strings.HasPrefix(strings.ToLower(dir), strings.ToLower(importPkgDir)) || strings.HasPrefix(strings.ToLower(importPkgDir), strings.ToLower(dir)) {
		appendTo(2)
		return
	}

	// vendor
	appendTo(1)
	return
}
