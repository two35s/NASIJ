package ast

import "fmt"

type ImportRecord struct {
	Source    string
	Specifier string
	Local     string
	IsDefault bool
	IsAll     bool
}

type ExportRecord struct {
	LocalName  string
	ExportName string
	Source     string
}

type ResolvedImport struct {
	Record   ImportRecord
	SourceAST *Program
}

type ImportResolver struct {
	Program    *Program
	Imports    []ImportRecord
	Exports    []ExportRecord
	Resolved   []ResolvedImport
	ScopeAnalysis *ScopeAnalysis
	errors     []error
}

func NewImportResolver(prog *Program, sa *ScopeAnalysis) *ImportResolver {
	return &ImportResolver{
		Program:       prog,
		ScopeAnalysis: sa,
	}
}

func (r *ImportResolver) Resolve() []error {
	r.collectImports()
	r.collectExports()
	return r.errors
}

func (r *ImportResolver) collectImports() {
	Walk(r.Program, func(n Node) VisitorFunc {
		imp, ok := n.(*ImportDeclaration)
		if !ok {
			return nil
		}
		for _, spec := range imp.Specifiers {
			rec := ImportRecord{
				Source: imp.Source.Value,
			}
			if spec.Default {
				rec.IsDefault = true
				if spec.Local != nil {
					rec.Local = spec.Local.Name
				}
			} else if spec.Local != nil {
				rec.Local = spec.Local.Name
				if spec.Imported != nil {
					rec.Specifier = spec.Imported.Name
				}
			}
			if spec.Imported == nil && !spec.Default {
				rec.IsAll = true
			}
			r.Imports = append(r.Imports, rec)
		}
		return nil
	})
}

func (r *ImportResolver) collectExports() {
	Walk(r.Program, func(n Node) VisitorFunc {
		exp, ok := n.(*ExportDeclaration)
		if !ok {
			return nil
		}
		if exp.Source != nil {
			for _, spec := range exp.Specifiers {
				r.Exports = append(r.Exports, ExportRecord{
					LocalName:  spec.Local.Name,
					ExportName: exportName(spec),
					Source:     exp.Source.Value,
				})
			}
			if exp.All {
				r.Exports = append(r.Exports, ExportRecord{
					Source: exp.Source.Value,
				})
			}
		} else if exp.Declaration != nil {
			name := exportedName(exp.Declaration)
			en := name
			if exp.Default {
				en = "default"
			}
			r.Exports = append(r.Exports, ExportRecord{
				LocalName:  name,
				ExportName: en,
			})
		} else {
			for _, spec := range exp.Specifiers {
				r.Exports = append(r.Exports, ExportRecord{
					LocalName:  spec.Local.Name,
					ExportName: exportName(spec),
				})
			}
		}
		return nil
	})
}

func exportName(spec ExportSpecifier) string {
	if spec.Exported != nil {
		return spec.Exported.Name
	}
	return spec.Local.Name
}

func exportedName(decl Statement) string {
	switch d := decl.(type) {
	case *FunctionDeclaration:
		if d.Name != nil {
			return d.Name.Name
		}
	case *ClassDeclaration:
		return d.Name.Name
	case *VariableDeclaration:
		if len(d.Decls) > 0 {
			return d.Decls[0].Name.Name
		}
	}
	return ""
}

func (r *ImportResolver) LookupLocal(localName string) *ImportRecord {
	for _, imp := range r.Imports {
		if imp.Local == localName {
			return &imp
		}
	}
	return nil
}

type ResolveError struct {
	Msg string
}

func (e *ResolveError) Error() string { return e.Msg }

func IsResolveError(err error) bool {
	_, ok := err.(*ResolveError)
	return ok
}

func NewResolveError(format string, args ...interface{}) error {
	return &ResolveError{Msg: fmt.Sprintf(format, args...)}
}
