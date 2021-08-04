package generator

import (
	"fmt"
	"strings"

	"github.com/atombender/go-jsonschema/pkg/codegen"
)

type validator interface {
	generate(out *codegen.Emitter)
	desc() *validatorDesc
}

type validatorDesc struct {
	hasError            bool
	beforeJSONUnmarshal bool
}

var (
	_ validator = new(requiredValidator)
	_ validator = new(nullTypeValidator)
	_ validator = new(defaultValidator)
)

type requiredValidator struct {
	jsonName string
}

func (v *requiredValidator) generate(out *codegen.Emitter) {
	out.Println(`if v, ok := %s["%s"]; !ok || v == nil {`, varNameRawMap, v.jsonName)
	out.Indent(1)
	out.Println(`return fmt.Errorf("field %s: required")`, v.jsonName)
	out.Indent(-1)
	out.Println("}")
}

func (v *requiredValidator) desc() *validatorDesc {
	return &validatorDesc{
		hasError:            true,
		beforeJSONUnmarshal: true,
	}
}

type nullTypeValidator struct {
	jsonName   string
	fieldName  string
	arrayDepth int
}

func (v *nullTypeValidator) generate(out *codegen.Emitter) {
	value := fmt.Sprintf("%s.%s", varNamePlainStruct, v.fieldName)
	fieldName := v.jsonName
	var indexes []string
	for i := 0; i < v.arrayDepth; i++ {
		index := fmt.Sprintf("i%d", i)
		indexes = append(indexes, index)
		out.Println(`for %s := range %s {`, index, value)
		value += fmt.Sprintf("[%s]", index)
		fieldName += "[%d]"
		out.Indent(1)
	}

	fieldName = fmt.Sprintf(`"%s"`, fieldName)
	if len(indexes) > 0 {
		fieldName = fmt.Sprintf(`fmt.Sprinf(%s, %s)`, fieldName, strings.Join(indexes, ", "))
	}

	out.Println(`if %s != nil {`, value)
	out.Indent(1)
	out.Println(`return fmt.Errorf("field %%s: must be null", %s)`, fieldName)
	out.Indent(-1)
	out.Println("}")

	for i := 0; i < v.arrayDepth; i++ {
		out.Indent(-1)
		out.Println("}")
	}
}

func (v *nullTypeValidator) desc() *validatorDesc {
	return &validatorDesc{
		hasError:            true,
		beforeJSONUnmarshal: false,
	}
}

type defaultValidator struct {
	jsonName     string
	fieldName    string
	defaultValue string
}

func (v *defaultValidator) generate(out *codegen.Emitter) {
	out.Println(`if v, ok := %s["%s"]; !ok || v == nil {`, varNameRawMap, v.jsonName)
	out.Indent(1)
	out.Println(`%s.%s = %s`, varNamePlainStruct, v.fieldName, v.defaultValue)
	out.Indent(-1)
	out.Println("}")
}

func (v *defaultValidator) desc() *validatorDesc {
	return &validatorDesc{
		hasError:            false,
		beforeJSONUnmarshal: false,
	}
}
