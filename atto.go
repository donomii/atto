package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strconv"

	"github.com/mattn/go-shellwords"
)

func makeOneArg(fn string, arg1 interface{}) []interface{} {
	out := []interface{}{fn}
	//out = append(out, fn)
	out = append(out, arg1)
	return out
}

func makeTwoArg(fn string, arg1, arg2 interface{}) []interface{} {
	return append(makeOneArg(fn, arg1), arg2)
}

func makeThreeArg(fn string, arg1, arg2, arg3 interface{}) []interface{} {
	return append(makeTwoArg(fn, arg1, arg2), arg3)

}

func searchTo(search string, tokens, accum []string) ([]string, []string) {
	if len(tokens) == 0 {
		return accum, tokens
	}
	if search == tokens[0] {
		return accum, tokens
	}
	return searchTo(search, tokens[1:], append(accum, tokens[0]))
}

func cloneEnv(e map[string][]interface{}) map[string][]interface{} {
	out := map[string][]interface{}{}
	for k, _ := range e {
		out[k] = e[k]
	}
	return out
}
func eval(expr interface{}, funcsmap map[string][]interface{}, args map[string]interface{}) interface{} {

	fnlist := expr.([]interface{})
	log.Printf("Evaluating: %+v\n", fnlist)
	fn := fnlist[0].(string)
	switch fn {

	case "if":
		if eval(fnlist[1], funcsmap, args).(bool) {
			return eval(fnlist[2], funcsmap, args)
		} else {
			return eval(fnlist[3], funcsmap, args)
		}
	case "__add":
		a, _ := strconv.ParseFloat(eval(fnlist[1], funcsmap, args).(string), 64)
		b, _ := strconv.ParseFloat(eval(fnlist[2], funcsmap, args).(string), 64)
		return fmt.Sprintf("%v", a+b)
	case "__strconcat":
		log.Printf("Concatenating strings: %v, %v\n", fnlist[1], fnlist[2])
		a := eval(fnlist[1], funcsmap, args)
		b := eval(fnlist[2], funcsmap, args)
		out := fmt.Sprintf("%+v%+v", a, b)
		log.Printf("Concatenated string: %v\n", out)
		return out
	case "__neg":
		a, _ := strconv.ParseFloat(eval(fnlist[1], funcsmap, args).(string), 64)
		return fmt.Sprintf("%v", -a)
	case "__str":
		return fmt.Sprintf("%v", fnlist[1])
	case "__eq":
		a := eval(fnlist[1], funcsmap, args)
		b := eval(fnlist[2], funcsmap, args)
		return reflect.DeepEqual(a, b)

	case "__print":
		out := fmt.Sprintf("%+v", eval(fnlist[1], funcsmap, args))
		fmt.Printf("%v\n", out)
		return out
	case "__value":
		val, ok := funcsmap[fnlist[1].(string)]
		if ok {
			log.Printf("Resolved to %v\n", val[2])
			return val[2]
		} else {
			log.Printf("Resolved to %v\n", fnlist[1])
			return fnlist[1]
		}
	case "__head":
		return fnlist[1].([]interface{})[0]
	case "__tail":
		return fnlist[1].([]interface{})[1:]
	case "__pair":
		car := eval(fnlist[1], funcsmap, args)
		cdr := eval(fnlist[2], funcsmap, args).(interface{})
		out := []interface{}{car, cdr}
		return out
	}

	userFunc, ok := funcsmap[fn]
	if ok {
		log.Printf("Userfunc: %+v\n", userFunc)
		newFM := cloneEnv(funcsmap)

		for i, v := range userFunc[1].([]string) {
			val := eval(expr.([]interface{})[1+i], funcsmap, args)
			log.Printf("Setting %v to %v\n", v, val)
			newFM[v] = []interface{}{v, []string{}, val}
		}
		return eval(userFunc[2], newFM, args)
	}

	log.Printf("Undefined function: %v\n", fnlist[0])
	os.Exit(1)
	return fnlist[0]
}

func parse_expr(tokens []string, args []string, func_defs map[string][]interface{}) ([]interface{}, []string) {

	if len(tokens) == 0 {
		return nil, tokens
	}
	token := tokens[0]

	log.Printf("Considering %v\n", token)

	if token == "fn" {
		name := tokens[1]
		remainder := tokens[2:]
		args, remainder := searchTo("is", remainder, []string{})

		log.Println("Found args", args)
		remainder = remainder[1:]
		body, remainder := searchTo("fn", remainder, []string{})
		log.Println("Found body", body)
		bodyTree, _ := parse_expr(body, args, func_defs)
		log.Println("Parsed body", bodyTree)
		log.Printf("Registering %v(%v)\n", name, args)

		func_defs[name] = makeTwoArg("fn", args, bodyTree)
		log.Printf("Registered functions %+v", func_defs)
		return parse_expr(remainder, args, func_defs)
	}

	if token == "if" {
		arg1, remainder := parse_expr(tokens[1:], args, func_defs)
		arg2, remainder := parse_expr(remainder, args, func_defs)
		arg3, remainder := parse_expr(remainder, args, func_defs)
		return makeThreeArg("if", arg1, arg2, arg3), remainder
	}

	//Single arg functions
	for _, builtIn := range []string{"__head", "__tail", "__litr", "__str", "__words", "__input", "__print", "__neg"} {
		if token == builtIn {
			arg1, remainder := parse_expr(tokens[1:], args, func_defs)
			log.Printf("Function %v(%v)\n", token, arg1)
			return makeOneArg(builtIn, arg1), remainder
		}
	}

	for _, builtIn := range []string{"__strconcat", "__fuse", "__pair", "__eq", "__add", "__mul", "__div", "__rem", "__less", "__lesseq"} {
		if token == builtIn {
			arg1, remainder := parse_expr(tokens[1:], args, func_defs)
			arg2, remainder := parse_expr(remainder, args, func_defs)
			log.Printf("Function %v(%v,%v)\n", token, arg1, arg2)
			return makeTwoArg(builtIn, arg1, arg2), remainder
		}
	}

	fn, ok := func_defs[token]
	remainder := tokens[1:]
	if ok {
		log.Printf(
			"Found defined function %v, num args: %+v, args: %+v,\n%+v\n",
			token,
			len(fn[1].([]string)),
			fn[1],
			fn,
		)
		out := []interface{}{}
		out = append(out, token)
		var next interface{}
		for range fn[1].([]string) {
			next, remainder = parse_expr(remainder, args, func_defs)
			out = append(out, next)
		}
		return out, remainder
	}

	//Default: it's a literal value
	log.Println("Literal:", token)
	return makeOneArg("__value", token), tokens[1:]
}

func lex(code string) []string {
	tokens, _ := shellwords.Parse(code)
	return tokens
}

func include_str(filename string) string {
	code, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	return string(code)
}
func with_core(code string) string {
	return include_str("core.at") + code
}

func exec(fname string) {
	code, _ := ioutil.ReadFile(fname)

	lexed := lex(with_core(string(code)))
	funcs := map[string][]interface{}{}

	//Parse twice to pick up forwards declarations.  The first pass parses the function args correctly,
	//but can't resolve any functions that are defined later in the file.
	//At the start of the second pass, we know the number of arguments for every function, so now we can
	//parse the bodies correctly
	parse_expr(lexed, []string{}, funcs) //First pass picks up function names
	parse_expr(lexed, []string{}, funcs) //Second pass gets function bodies correctly

	main, ok := funcs["main"]
	log.Printf("Registered functions %+v", funcs)
	if ok {
		eval(main[2], funcs, nil)
	}

}

func main() {
	exec(os.Args[1])
}