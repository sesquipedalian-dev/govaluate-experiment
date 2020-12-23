package main

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"

	"github.com/Knetic/govaluate"
	"github.com/fatih/structs"
)

/* with thanks to https://github.com/casbin/casbin/blob/d0d65d828c784211a47c23f9cc63b7429da0287e/util/builtin_operators.go */
// validate the variadic parameter size and type as string
func validateVariadicArgs(expectedLen int, args ...interface{}) error {
	if len(args) != expectedLen {
		return fmt.Errorf("Expected %d arguments, but got %d", expectedLen, len(args))
	}

	for _, p := range args {
		_, ok := p.(string)
		if !ok {
			return errors.New("Argument must be a string")
		}
	}

	return nil
}

// RegexMatch determines whether key1 matches the pattern of key2 in regular expression.
func RegexMatch(key1 string, key2 string) bool {
	res, err := regexp.MatchString(key2, key1)
	if err != nil {
		panic(err)
	}
	return res
}

// RegexMatchFunc is the wrapper for RegexMatch.
func RegexMatchFunc(args ...interface{}) (interface{}, error) {
	if err := validateVariadicArgs(2, args...); err != nil {
		return false, fmt.Errorf("%s: %s", "regexMatch", err)
	}

	name1 := args[0].(string)
	name2 := args[1].(string)

	return bool(RegexMatch(name1, name2)), nil
}

/* End thanks */

// THIS got weird when the params were in the other order (e.g. array, regex)
// in this case it flattened the array into the args, e.g.
// Any([1,2], /\d+/)
// args = 1,2,/\d+/
// putting it in regex, array order does not flatten it
// args = /\d+/, [1,2]
//
func Any(args ...interface{}) (interface{}, error) {
	if len(args) < 2 {
		return false, fmt.Errorf("Expected 2+ arguments, but got %d", len(args))
	}

	function, ok := args[0].(string)
	if !ok {
		return false, fmt.Errorf("arg 0 is not a string!")
	}

	arr, ok := args[1].([]interface{})
	if !ok {
		return false, fmt.Errorf("arg 1 is not a slice! ***%v*** %v", args, reflect.TypeOf(args[0]))
	}

	functions := map[string]govaluate.ExpressionFunction{
		"regexMatch": RegexMatchFunc,
	}
	expression, err := govaluate.NewEvaluableExpressionWithFunctions(function, functions)
	if err != nil {
		panic(err)
	}

	found := false
	for _, item := range arr {
		params, ok := item.(map[string]interface{})
		if !ok {
			return false, fmt.Errorf("item is not a map %v", item)
		}
		fmt.Printf("sending params in any %v\n", params)
		result, err := expression.Evaluate(params)
		if err != nil {
			panic(err)
		}

		if boolResult, ok := result.(bool); ok {
			if boolResult {
				found = true
				break
			}
		}
	}
	return found, nil
}

/* analogs to flagr structs we want to validate */
type Tag struct {
	ID    int64
	Value string
}

type Segment struct {
	ID             int64
	RolloutPercent int64
}

type Flag struct {
	Tags     []Tag
	ID       int64
	Key      string
	Segments []Segment
}

/* end struct analogs */

func main() {
	fmt.Printf("Hello, World! %v\n", "nothing")

	expressionString := `regexMatch(tag, "^JIRA:[a-zA-Z]{3}[a-zA-Z]*$")`
	functions := map[string]govaluate.ExpressionFunction{
		"regexMatch": RegexMatchFunc,
		"any":        Any,
	}
	expression, err := govaluate.NewEvaluableExpressionWithFunctions(expressionString, functions)
	if err != nil {
		panic(err)
	}

	tagExamples := []string{
		"JIRA:EPLT",
		"FOO:BAR",
		"JIRA:TS",
		"JIRA:TLA",
		"JIRA:EP123",
		"EXTRA_STUFF kljalkj JIRA:EPLT EXTRA",
	}
	for _, tag := range tagExamples {
		params := map[string]interface{}{
			"tag": tag,
		}
		result, err := expression.Evaluate(params)
		if err != nil {
			panic(err)
		}
		fmt.Printf("is tag (%s) valid? %t\n", tag, result)
	}

	structExpressionStrings := []string{
		`any("regexMatch(Value, \"^JIRA:[a-zA-Z]{3}[a-zA-Z]*$\")", Tags) `,
		`
		any("regexMatch(Value, \"^JIRA:[a-zA-Z]{3}[a-zA-Z]*$\")", Tags) &&
		any("RolloutPercent > 10", Segments)
		`,
	}

	structExpressions := make([]*govaluate.EvaluableExpression, 0)
	for _, structExpressionString := range structExpressionStrings {
		expression, err = govaluate.NewEvaluableExpressionWithFunctions(structExpressionString, functions)
		if expression == nil || err != nil {
			panic(err)
		}
		structExpressions = append(structExpressions, expression)
	}

	structExamples := []Flag{
		Flag{
			ID: 1,
		}, // invalid, no tags
		Flag{
			ID: 2,
			Tags: []Tag{
				Tag{
					ID:    3,
					Value: "FOO:BAR", // invalid
				},
				Tag{
					ID:    6,
					Value: "JIRA:EPLT", // valid
				},
			},
		},
		Flag{
			ID: 4,
			Tags: []Tag{
				Tag{
					ID:    5,
					Value: "JIRA:EPLT", // valid
				},
			},
		},
		Flag{
			ID: 7,
			Tags: []Tag{
				Tag{
					ID:    8,
					Value: "JIRA:EP12", //invalid
				},
			},
		},
		Flag{
			ID: 9,
			Tags: []Tag{
				Tag{
					ID:    10,
					Value: "JIRA:EPLT", // valid
				},
			},
			Segments: []Segment{
				Segment{
					ID:             11,
					RolloutPercent: 20,
				},
			},
		},
		Flag{
			ID: 12,
			Tags: []Tag{
				Tag{
					ID:    13,
					Value: "JIRA:EPLT", // valid
				},
			},
			Segments: []Segment{
				Segment{
					ID:             14,
					RolloutPercent: 5,
				},
			},
		},
	}

	for expressionIndex, structExpression := range structExpressions {
		for _, flag := range structExamples {
			// map := structs.Map(structExample)

			params := structs.Map(flag)
			fmt.Printf("params arg %v %v %v %v\n", params, params["Tags"], reflect.TypeOf(params["Tags"]), structExpression)
			result, err := structExpression.Evaluate(params)
			if err != nil {
				panic(err)
			}
			fmt.Printf("is flag (%v) valid? for exp %s, %t\n", structExpressionStrings[expressionIndex], flag, result)

		}
	}
}
