package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"reflect"
	"strings"
)

type codegen struct {
	PackageName string
	Recvs       map[string][]handler
}
type handler struct {
	Name     string
	Params   handlerParams
	InStruct inStruct
}

type inStruct struct {
	Type   string
	Fields []field
}

type field struct {
	Name            string
	Type            string
	ValidationRules []validationRule
}

type validationRule struct {
	Type  string
	Value string
}

type handlerParams struct {
	URL    string `json:"url"`
	Auth   bool   `json:"auth"`
	Method string `json:"method"`
}

var (
	auth = `	if r.Header.Get("X-Auth") != "100500" {
		return ApiError{http.StatusForbidden, errors.New("unauthorized")}
	}
`
	checkHTTPMethod = `	if r.Method != "%v" {
		return ApiError{http.StatusNotAcceptable, errors.New("bad method")}
	}
`
	handleErr = `	if err != nil {
		aerr, ok := err.(ApiError)
		if !ok {
			http.Error(w, "{\"error\":\"" + err.Error() + "\"}", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(aerr.HTTPStatus)
		w.Write([]byte("{\"error\":\"" + aerr.Error() + "\"}"))
		return
	}`

	imports = `import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"errors"
)
	`
)

func main() {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	out, _ := os.Create(os.Args[2])

	codegen, err := getCodeToGenerate(node)
	if err != nil {
		log.Fatal(err)
	}

	//generating code
	codeGenerate(out, codegen)
}

func getCodeToGenerate(node *ast.File) (codegen, error) {
	cg := codegen{
		PackageName: node.Name.Name,
		Recvs:       map[string][]handler{},
	}

	for _, f := range node.Decls {
		g, ok := f.(*ast.FuncDecl)
		if !ok {
			//fmt.Printf("SKIP %T is not *ast.GenDecl\n", f)
			continue
		}

		if g.Doc == nil {
			//fmt.Printf("SKIP method %#v doesnt have comments\n", g.Name)
			continue
		}

		if g.Recv == nil {
			//fmt.Printf("SKIP method %#v doesnt have recv\n", g.Name)
			continue
		}

		needCodegen := false
		var params handlerParams
		prefix := "// apigen:api"
		for _, comment := range g.Doc.List {
			if strings.HasPrefix(comment.Text, prefix) {
				needCodegen = true
				jsonParams := strings.TrimSpace(strings.TrimPrefix(comment.Text, prefix))
				if jsonParams == "" {
					break
				}
				json.Unmarshal([]byte(jsonParams), &params)
				break
			}
		}
		if !needCodegen {
			//fmt.Printf("SKIP method %#v doesnt have apigen mark\n", g.Name)
			continue
		}

		if len(g.Recv.List) > 1 {
			//fmt.Printf("SKIP method %#v unpredictable recv number\n", g.Name)
			continue
		}

		for _, p := range g.Recv.List {
			pv, ok := p.Type.(*ast.StarExpr)
			if !ok {
				return cg, fmt.Errorf("not such type: %s", p.Type)
			}
			si, ok := pv.X.(*ast.Ident)
			if !ok {
				return cg, fmt.Errorf("not such type: %s", p.Type)
			}

			if len(g.Type.Params.List) != 2 {
				//fmt.Printf("SKIP method %#v unknown in params structure\n", g.Name)
				continue
			}

			paramType := g.Type.Params.List[1].Type.(*ast.Ident).Name

			fields := structFields(node, paramType)

			cg.Recvs[si.Name] = append(cg.Recvs[si.Name], handler{
				Name:   g.Name.Name,
				Params: params,
				InStruct: inStruct{
					Type:   paramType,
					Fields: fields,
				},
			})

		}

	}
	return cg, nil
}

func codeGenerate(out *os.File, gen codegen) {
	//start generating
	fmt.Fprintln(out, `package `+gen.PackageName)
	fmt.Fprintln(out) // empty line
	fmt.Fprintln(out, imports)
	fmt.Fprintln(out) // empty line

	var recvNames []string
	for name := range gen.Recvs {
		recvNames = append(recvNames, name)
	}

	for _, name := range recvNames {
		fmt.Fprintln(out, fmt.Sprintf("func (srv *%v) ServeHTTP(w http.ResponseWriter, r *http.Request) {", name))
		fmt.Fprintln(out, "\tw.Header().Set(\"Content-type\", \"application/json\")")
		fmt.Fprintln(out, "\tvar err error")
		fmt.Fprintln(out, "\tswitch r.URL.Path {")
		for _, handler := range gen.Recvs[name] {
			fmt.Fprintf(out, "\tcase \"%v\":\n", handler.Params.URL)
			fmt.Fprintf(out, "\t\terr = srv.handler%v(w, r)\n", handler.Name)
		}
		fmt.Fprintln(out, "\tdefault:")
		fmt.Fprintln(out, "\t\terr = ApiError{http.StatusNotFound, errors.New(\"unknown method\")}")
		fmt.Fprintln(out, "\t}")

		fmt.Fprintln(out, handleErr)

		fmt.Fprintln(out, "\tw.WriteHeader(http.StatusOK)")
		fmt.Fprintln(out, "}")
		fmt.Fprintln(out, "")

		for _, handler := range gen.Recvs[name] {
			fmt.Fprintf(out, "// %s %s handler\n", handler.Name, name)
			fmt.Fprintf(out, "func (srv *%s) handler%s(w http.ResponseWriter, r *http.Request) error {\n", name, handler.Name)

			//check method
			if handler.Params.Method != "" {
				fmt.Fprintf(out, checkHTTPMethod, handler.Params.Method)
			}

			//auth
			if handler.Params.Auth {
				fmt.Fprint(out, auth)
			}

			//getting params from url query
			for _, field := range handler.InStruct.Fields {
				queryName, paramName := strings.ToLower(field.Name), strings.ToLower(field.Name)
				if v, ok := isSpecificParamName(field.ValidationRules); ok {
					queryName = v
				}
				fmt.Fprintf(out, "\tvar %sStr string\n", paramName)
				fmt.Fprintln(out, "\tswitch r.Method {")
				fmt.Fprintln(out, "\tcase \"GET\":")
				fmt.Fprintf(out, "\t\t%sKeys, ok := r.URL.Query()[\"%s\"]\n", paramName, queryName)
				fmt.Fprintf(out, "\t\tif ok && len(%sKeys) > 0 {\n", paramName)
				fmt.Fprintf(out, "\t\t\t%sStr = %sKeys[0]\n", paramName, paramName)
				fmt.Fprintln(out, "\t\t}")
				fmt.Fprintln(out, "\tcase \"POST\":")
				fmt.Fprintf(out, "\t\t%sStr = r.FormValue(\"%s\")\n", paramName, queryName)
				fmt.Fprintln(out, "\t}")
				switch field.Type {
				case "int":
					fmt.Fprintf(out, "\t%sInt, err := strconv.Atoi(%sStr)\n", paramName, paramName)
					fmt.Fprintf(out, `	if err != nil {
		return ApiError{http.StatusBadRequest, errors.New("%s must be int")}
	}`, paramName)
					fmt.Fprintf(out, "\n\t%s := %sInt\n", paramName, paramName)
				case "string":
					fmt.Fprintf(out, "\t%s := %sStr\n", paramName, paramName)
				}

				//validation
				for _, vRule := range field.ValidationRules {
					fmt.Fprintln(out, validationRuleStr(vRule, field.Type, paramName))
					//fmt.Fprintln(out, "")
				}
			}

			//assigment
			fmt.Fprintf(out, "\tparams := %s{\n", handler.InStruct.Type)
			for _, field := range handler.InStruct.Fields {
				fmt.Fprintf(out, "\t\t%s:%s,\n", field.Name, strings.ToLower(field.Name))
			}
			fmt.Fprintln(out, "\t}")

			//
			fmt.Fprintf(out, `	ctx := context.Background()
	res, err := srv.%s(ctx, params)
	if err != nil {
		return err
	}
	resp := map[string]interface{}{
		"error": "",
		"response": res,
	}
	json.NewEncoder(w).Encode(resp)
`, handler.Name)
			fmt.Fprintln(out, "\treturn nil")
			fmt.Fprintln(out, "}")
			fmt.Fprintln(out, "")
		}
	}
}

func structFields(node *ast.File, paramType string) []field {
	var fields []field
	for _, f := range node.Decls {
		g, ok := f.(*ast.GenDecl)
		if !ok {
			//fmt.Printf("SKIP %T is not *ast.GenDecl\n", f)
			continue
		}
		for _, spec := range g.Specs {
			currType, ok := spec.(*ast.TypeSpec)
			if !ok {
				//fmt.Printf("SKIP %T is not ast.TypeSpec\n", spec)
				continue
			}

			currStruct, ok := currType.Type.(*ast.StructType)
			if !ok {
				//fmt.Printf("SKIP %T is not ast.StructType\n", currStruct)
				continue
			}

			if currType.Name.Name == paramType {
				for _, f := range currStruct.Fields.List {
					if f.Tag == nil {
						continue
					}
					tag := reflect.StructTag(f.Tag.Value[1 : len(f.Tag.Value)-1])
					rules := fieldValidationRules(tag.Get("apivalidator"))

					fieldName := f.Names[0].Name
					fieldType := f.Type.(*ast.Ident).Name

					fields = append(fields, field{
						Name:            fieldName,
						Type:            fieldType,
						ValidationRules: rules,
					})
				}
			}
		}
	}
	return fields
}

func fieldValidationRules(rule string) []validationRule {
	var validationRules []validationRule
	if rule == "" {
		return validationRules
	}
	rules := strings.Split(rule, ",")
	for _, r := range rules {
		var value string
		rParts := strings.Split(r, "=")
		if len(rParts) == 2 {
			value = rParts[1]
		}
		validationRules = append(validationRules, validationRule{
			Type:  rParts[0],
			Value: value,
		})
	}
	return validationRules
}

func isSpecificParamName(valids []validationRule) (string, bool) {
	for _, v := range valids {
		if v.Type == "paramname" {
			return v.Value, true
		}
	}
	return "", false
}

func validationRuleStr(validation validationRule, vType, vName string) string {
	switch validation.Type {
	case "required":
		return requiredRule(vType, vName)
	case "enum":
		return enumRule(validation.Value, vName)
	case "default":
		return defaultRule(validation.Value, vType, vName)
	case "min":
		return minRule(validation.Value, vType, vName)
	case "max":
		return maxRule(validation.Value, vType, vName)
	}
	return ""
}

func requiredRule(vType, vName string) (rule string) {
	switch vType {
	case "int":
		rule = fmt.Sprintf(`	if %s == 0 {
		return ApiError{http.StatusBadRequest, errors.New("%s must be not empty")}
	}`, vName, vName)
	case "string":
		rule = fmt.Sprintf(`	if %s == "" {
		return ApiError{http.StatusBadRequest, errors.New("%s must be not empty")}
	}`, vName, vName)
	}
	return
}

func enumRule(ruleValue, vName string) (rule string) {
	strs := strings.Split(ruleValue, "|")
	var rules []string
	for _, s := range strs {
		rules = append(rules, fmt.Sprintf("%s != \"%s\"", vName, s))
	}
	result := strings.Join(rules, " && ")
	rule = fmt.Sprintf(`	if %s != "" {
		if %s {
			return ApiError{http.StatusBadRequest, errors.New("%s must be one of [%s]")}
		}
	}`, vName, result, vName, strings.Join(strs, ", "))
	return
}

func defaultRule(ruleValue, vType, vName string) (rule string) {
	switch vType {
	case "int":
		rule = fmt.Sprintf(`	if %s == 0 {
		%s = "%s"
	}`, vName, vName, ruleValue)
	case "string":
		rule = fmt.Sprintf(`	if %s == "" {
		%s = "%s"
	}`, vName, vName, ruleValue)
	}
	return
}

func minRule(ruleValue, vType, vName string) (rule string) {
	switch vType {
	case "int":
		rule = fmt.Sprintf(`	if %s < %s {
		return ApiError{http.StatusBadRequest, errors.New("%s must be >= %s")}
	}`, vName, ruleValue, vName, ruleValue)
	case "string":
		rule = fmt.Sprintf(`
	if len(%s) < %s {
		return ApiError{http.StatusBadRequest, errors.New("%s len must be >= %s")}
	}`, vName, ruleValue, vName, ruleValue)
	}
	return
}

func maxRule(ruleValue, vType, vName string) (rule string) {
	switch vType {
	case "int":
		rule = fmt.Sprintf(`	if %s > %s {
		return ApiError{http.StatusBadRequest, errors.New("%s must be <= %s")}
	}`, vName, ruleValue, vName, ruleValue)
	case "string":
		rule = fmt.Sprintf(`	if len(%s) > %s {
		return ApiError{http.StatusBadRequest, errors.New("%s len must be <= %s")}
	}`, vName, ruleValue, vName, ruleValue)
	}
	return
}
