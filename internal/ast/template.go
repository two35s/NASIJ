package ast

type TemplateResolver struct {
	Program *Program
}

func NewTemplateResolver(prog *Program) *TemplateResolver {
	return &TemplateResolver{Program: prog}
}

func (tr *TemplateResolver) ResolveAll() []TemplateCall {
	var calls []TemplateCall
	seenQuasis := make(map[*TemplateLiteral]bool)
	Walk(tr.Program, func(n Node) VisitorFunc {
		switch n := n.(type) {
		case *TaggedTemplateExpression:
			seenQuasis[n.Quasi] = true
			info := tr.resolveTaggedTemplate(n)
			if info != nil {
				calls = append(calls, *info)
			}
		case *TemplateLiteral:
			if seenQuasis[n] {
				return nil
			}
			info := tr.resolveTemplateLiteral(n)
			if info != nil {
				calls = append(calls, *info)
			}
		}
		return nil
	})
	return calls
}

type TemplateCall struct {
	Tag    string
	Parts  []string
	Exprs  []Expression
	IsTagged bool
	Pos    Pos
}

func (tr *TemplateResolver) resolveTemplateLiteral(tl *TemplateLiteral) *TemplateCall {
	if len(tl.Parts) == 0 {
		return nil
	}
	tc := &TemplateCall{}
	for _, p := range tl.Parts {
		if p.Expr != nil {
			tc.Exprs = append(tc.Exprs, p.Expr)
		}
		tc.Parts = append(tc.Parts, p.Value)
	}
	tc.Pos = tl.StartPos
	return tc
}

func (tr *TemplateResolver) resolveTaggedTemplate(tte *TaggedTemplateExpression) *TemplateCall {
	tag := ""
	if id, ok := tte.Tag.(*Identifier); ok {
		tag = id.Name
	} else if me, ok := tte.Tag.(*MemberExpression); ok {
		if obj, ok := me.Object.(*Identifier); ok {
			tag = obj.Name + "." + me.Property.String()
		}
	}
	tc := tr.resolveTemplateLiteral(tte.Quasi)
	if tc == nil {
		return nil
	}
	tc.Tag = tag
	tc.IsTagged = true
	tc.Pos = tte.StartPos
	return tc
}

func (tr *TemplateResolver) IsStatic(tl *TemplateLiteral) bool {
	for _, p := range tl.Parts {
		if p.Expr != nil {
			return false
		}
	}
	return true
}

func (tr *TemplateResolver) ConcatStatic(tl *TemplateLiteral) string {
	var s string
	for _, p := range tl.Parts {
		s += p.Value
	}
	return s
}
