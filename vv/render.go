package vv

import (
	"bytes"
	"fmt"
	"html/template"
	"monkey/ast"
	"monkey/lexer"
	"monkey/parser"
	"reflect"

	"github.com/gobuffalo/velvet"
	"github.com/pkg/errors"
)

func Render(s string, ctx *velvet.Context) (string, error) {
	l := lexer.New(s)
	p := parser.New(l)
	prog := p.ParseProgram()

	ev := evaler{
		ctx:     ctx,
		program: prog,
	}

	return ev.eval()
}

type evaler struct {
	ctx     *velvet.Context
	program *ast.Program
}

type interfaceable interface {
	Interface() interface{}
}

func (ev *evaler) eval() (string, error) {
	bb := &bytes.Buffer{}
	for _, stmt := range ev.program.Statements {
		var res interface{}
		var err error
		switch node := stmt.(type) {
		case *ast.ExpressionStatement:
			res, err = ev.evalExpression(node.Expression)
		case *ast.LetStatement:
			res, err = ev.evalLetStatement(node)
		}
		if err != nil {
			return "", err
		}

		ev.write(bb, res)
	}
	return bb.String(), nil
}

func (ev *evaler) write(bb *bytes.Buffer, i interface{}) {
	switch t := i.(type) {
	case string, interfaceable:
		bb.WriteString(template.HTMLEscaper(t))
	case template.HTML:
		bb.WriteString(string(t))
	case int64, int, float64:
		bb.WriteString(fmt.Sprint(t))
	case []interface{}:
		for _, ii := range t {
			ev.write(bb, ii)
		}
	}
}

func (ev *evaler) evalExpression(node ast.Expression) (interface{}, error) {
	switch s := node.(type) {
	case *ast.HTMLLiteral:
		return template.HTML(s.Value), nil
	case *ast.StringLiteral:
		return s.Value, nil
	case *ast.IntegerLiteral:
		return s.Value, nil
	case *ast.FloatLiteral:
		return s.Value, nil
	case *ast.InfixExpression:
		return ev.evalInfixExpression(s)
	case *ast.Identifier:
		return ev.evalIdentifier(s)
	case *ast.HashLiteral:
		return ev.evalHashLiteral(s)
	case *ast.IndexExpression:
		return ev.evalIndexExpression(s)
	case *ast.CallExpression:
		return ev.evalCallExpression(s)
	case *ast.Boolean:
		return s.Value, nil
	case *ast.ArrayLiteral:
		return ev.evalArrayLiteral(s)
	case *ast.ForExpression:
		return ev.evalForExpression(s)
	case *ast.IfExpression:
		return ev.evalIfExpression(s)
	case nil:
		return nil, nil
	}
	return nil, errors.Errorf("could not evaluate node %T", node)
}

func (ev *evaler) evalIfExpression(node *ast.IfExpression) (interface{}, error) {
	c, err := ev.evalExpression(node.Condition)
	if err != nil {
		return nil, err
	}

	var r interface{}
	fmt.Printf("### ev.isTruthy(c) -> %+v\n", ev.isTruthy(c))
	if ev.isTruthy(c) {
		r, err = ev.evalBlockStatement(node.Consequence)
	} else {
		r, err = ev.evalBlockStatement(node.Alternative)
	}

	if err != nil {
		return nil, err
	}

	return r, nil
}

func (ev *evaler) isTruthy(i interface{}) bool {
	if i == nil {
		return false
	}
	if b, ok := i.(bool); ok {
		return b
	}
	return true
}

func (ev *evaler) evalIndexExpression(node *ast.IndexExpression) (interface{}, error) {
	index, err := ev.evalExpression(node.Index)
	if err != nil {
		return nil, err
	}
	left, err := ev.evalExpression(node.Left)
	if err != nil {
		return nil, err
	}
	rv := reflect.ValueOf(left)
	switch rv.Kind() {
	case reflect.Map:
		return rv.MapIndex(reflect.ValueOf(index)).Interface(), nil
	case reflect.Array, reflect.Slice:
		if i, ok := index.(int64); ok {
			return rv.Index(int(i)).Interface(), nil
		}
	}
	return nil, errors.Errorf("could not index %T with %T", left, index)
}

func (ev *evaler) evalHashLiteral(node *ast.HashLiteral) (interface{}, error) {
	m := map[string]interface{}{}
	for ke, ve := range node.Pairs {
		k, err := ev.evalExpression(ke)
		if err != nil {
			return nil, err
		}
		v, err := ev.evalExpression(ve)
		if err != nil {
			return nil, err
		}
		m[k.(string)] = v
	}
	return m, nil
}

func (ev *evaler) evalLetStatement(node *ast.LetStatement) (interface{}, error) {

	v, err := ev.evalExpression(node.Value)
	if err != nil {
		return nil, err
	}
	ev.ctx.Set(node.Name.Value, v)
	return nil, nil
}

func (ev *evaler) evalIdentifier(node *ast.Identifier) (interface{}, error) {
	return ev.ctx.Get(node.Value), nil
}

func (ev *evaler) evalInfixExpression(node *ast.InfixExpression) (interface{}, error) {
	lres, err := ev.evalExpression(node.Left)
	if err != nil {
		return nil, err
	}
	rres, err := ev.evalExpression(node.Right)
	if err != nil {
		return nil, err
	}

	switch t := lres.(type) {
	case string:
		return ev.stringsOperator(t, rres, node.Operator)
	case int64:
		if r, ok := rres.(int64); ok {
			return ev.intsOperator(t, r, node.Operator)
		}
	case float64:
		if r, ok := rres.(float64); ok {
			return ev.floatsOperator(t, r, node.Operator)
		}
	}
	return nil, errors.Errorf("unable to operate on %T and %T", lres, rres)
}

func (ev *evaler) intsOperator(l int64, r int64, op string) (interface{}, error) {
	switch op {
	case "+":
		return l + r, nil
	case "-":
		return l - r, nil
	case "/":
		return l / r, nil
	case "*":
		return l * r, nil
	}
	return nil, errors.Errorf("unknown operator for integer %s", op)
}

func (ev *evaler) floatsOperator(l float64, r float64, op string) (interface{}, error) {
	switch op {
	case "+":
		return l + r, nil
	case "-":
		return l - r, nil
	case "/":
		return l / r, nil
	case "*":
		return l * r, nil
	}
	return nil, errors.Errorf("unknown operator for float %s", op)
}

func (ev *evaler) stringsOperator(l string, r interface{}, op string) (interface{}, error) {
	if op == "+" {
		return l + fmt.Sprint(r), nil
	}
	return nil, errors.Errorf("unknown operator for string %s", op)
}

func (ev *evaler) evalCallExpression(node *ast.CallExpression) (interface{}, error) {
	var rv reflect.Value
	if node.Callee != nil {
		c, err := ev.evalExpression(node.Callee)
		if err != nil {
			return nil, err
		}
		rc := reflect.ValueOf(c)
		rv = rc.MethodByName(node.Function.String())
	} else {
		f, err := ev.evalExpression(node.Function)
		if err != nil {
			return nil, err
		}
		rv = reflect.ValueOf(f)
	}
	if !rv.IsValid() {
		return nil, errors.Errorf("%+v (%T) is an invalid function value", rv, rv)
	}

	args := []reflect.Value{}
	for _, a := range node.Arguments {
		v, err := ev.evalExpression(a)
		if err != nil {
			return nil, err
		}
		rv := reflect.ValueOf(v)
		if !rv.IsValid() {
			return nil, errors.Errorf("%+v (%T) is an invalid value", v, v)
		}
		args = append(args, rv)
	}

	res := rv.Call(args)
	if len(res) > 0 {
		if len(res) > 1 {
			if e, ok := res[1].Interface().(error); ok {
				return nil, e
			}
		}
		return res[0].Interface(), nil
	}
	return nil, nil
}

func (ev *evaler) evalForExpression(node *ast.ForExpression) (interface{}, error) {
	iter, err := ev.evalExpression(node.Iterable)
	if err != nil {
		return nil, err
	}
	riter := reflect.ValueOf(iter)
	ret := []interface{}{}
	switch riter.Kind() {
	case reflect.Map:
		octx := ev.ctx
		keys := riter.MapKeys()
		for i := 0; i < len(keys); i++ {
			k := keys[i]
			v := riter.MapIndex(k)
			ev.ctx = octx.New()
			ev.ctx.Set(node.KeyName, k.Interface())
			ev.ctx.Set(node.ValueName, v.Interface())
			res, err := ev.evalBlockStatement(node.Consequence)
			ev.ctx = octx
			if err != nil {
				return nil, err
			}
			ret = append(ret, res)
		}
	case reflect.Slice, reflect.Array:
		octx := ev.ctx
		for i := 0; i < riter.Len(); i++ {
			ev.ctx = octx.New()
			v := riter.Index(i)
			ev.ctx.Set(node.KeyName, i)
			ev.ctx.Set(node.ValueName, v.Interface())
			res, err := ev.evalBlockStatement(node.Consequence)
			ev.ctx = octx
			if err != nil {
				return nil, err
			}
			if res != nil {
				ret = append(ret, res)
			}
		}
	default:
		return ret, errors.Errorf("could not iterate over %T", iter)
	}
	return ret, nil
}

func (ev *evaler) evalBlockStatement(node *ast.BlockStatement) (interface{}, error) {
	fmt.Println("evalBlockStatement")
	res := []interface{}{}
	for _, s := range node.Statements {
		i, err := ev.evalStatement(s)
		if err != nil {
			return nil, err
		}
		if i != nil {
			res = append(res, i)
		}
	}
	return res, nil
}

func (ev *evaler) evalStatement(node ast.Statement) (interface{}, error) {
	fmt.Println("evalStatement")
	fmt.Printf("### node -> %T\n", node)
	switch t := node.(type) {
	case *ast.ExpressionStatement:
		s, err := ev.evalExpression(t.Expression)
		if _, ok := t.Expression.(*ast.HTMLLiteral); ok {
			return s, err
		}
		return nil, err
	case *ast.ReturnStatement:
		return ev.evalReturnStatement(t)
	}
	return nil, errors.Errorf("could not eval statement %T", node)
}

func (ev *evaler) evalReturnStatement(node *ast.ReturnStatement) (interface{}, error) {
	res, err := ev.evalExpression(node.ReturnValue)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (ev *evaler) evalArrayLiteral(node *ast.ArrayLiteral) (interface{}, error) {
	res := []interface{}{}
	for _, e := range node.Elements {
		i, err := ev.evalExpression(e)
		if err != nil {
			return nil, err
		}
		res = append(res, i)
	}
	return res, nil
}
