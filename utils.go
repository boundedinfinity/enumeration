package enumer

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

func GetName[E ~string]() string {
	return reflect.TypeOf(E("")).String()
}

func Join[A ~string](values []A, sep string) string {
	var ss []string

	for _, value := range values {
		ss = append(ss, string(value))
	}

	return strings.Join(ss, sep)
}

func IsEq[A ~string, B ~string](a A) func(B) bool {
	as := string(a)
	lower := strings.ToLower(as)
	upper := strings.ToLower(as)

	return func(b B) bool {
		bs := string(b)
		return as == bs ||
			lower == strings.ToLower(bs) ||
			upper == strings.ToUpper(bs)
	}
}

func MarshalJSON[E ~string](e E) ([]byte, error) {
	return json.Marshal(string(e))
}

func UnmarshalJSON[E ~string](data []byte, e *E, parser func(string) (E, error)) error {
	var s string

	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	p, err := parser(s)

	if err != nil {
		return err
	}

	*e = p

	return nil
}

func MarshalYAML[E ~string](e E) (interface{}, error) {
	return string(e), nil
}

func UnmarshalYAML[E ~string](unmarshal func(interface{}) error, e *E, parser func(string) (E, error)) error {
	var s string

	if err := unmarshal(&s); err != nil {
		return err
	}

	p, err := parser(s)

	if err != nil {
		return err
	}

	*e = p

	return nil
}

func Value[E ~string](e E) (driver.Value, error) {
	return string(e), nil
}

func Scan[E ~string](value interface{}, e *E, parser func(string) (E, error)) error {
	if value == nil {
		return fmt.Errorf("cannot be null")
	}

	dv, err := driver.String.ConvertValue(value)

	if err != nil {
		return err
	}

	s, ok := dv.(string)

	if !ok {
		return fmt.Errorf("not a string")
	}

	p, err := parser(s)

	if err != nil {
		return err
	}

	*e = p

	return nil
}