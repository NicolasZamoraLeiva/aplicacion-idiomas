package activity

import "strconv"

const (
	KindSelect = "select"
	KindInt    = "int"
)

type Option struct {
	Value string
	Label string
}

type Setting struct {
	Key     string
	Label   string
	Help    string
	Kind    string
	Options []Option
	Min     int
	Max     int
	Default string
}

type Info struct {
	ID   string
	Name string
}

type Activity interface {
	Info() Info
	Settings() []Setting
	Validate(values map[string]string) (map[string]string, error)
}

// Coerce fills defaults and drops invalid stored values so a bad row cannot
// break the next request.
func Coerce(defs []Setting, in map[string]string) map[string]string {
	out := make(map[string]string, len(defs))
	for _, def := range defs {
		raw, ok := in[def.Key]
		if !ok || !valid(def, raw) {
			out[def.Key] = def.Default
			continue
		}
		out[def.Key] = raw
	}
	return out
}

// Check accepts a submitted form. Every definition must be present and valid.
func Check(defs []Setting, in map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(defs))
	for _, def := range defs {
		raw, ok := in[def.Key]
		if !ok || !valid(def, raw) {
			return nil, errInvalid
		}
		out[def.Key] = raw
	}
	return out, nil
}

type invalidError struct{}

func (invalidError) Error() string { return "invalid settings" }

var errInvalid = invalidError{}

func IsInvalid(err error) bool {
	return err == errInvalid
}

func valid(def Setting, raw string) bool {
	switch def.Kind {
	case KindSelect:
		for _, opt := range def.Options {
			if opt.Value == raw {
				return true
			}
		}
		return false
	case KindInt:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return false
		}
		return n >= def.Min && n <= def.Max
	default:
		return false
	}
}
