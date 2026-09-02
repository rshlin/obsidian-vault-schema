package main

// PyYAML-compatible resolution of plain (unquoted) YAML scalars.
//
// Why this file exists at all: this tool is the optional second opinion on a
// vault whose PRIMARY validator is an in-tree Python script built on PyYAML.
// PyYAML implements YAML 1.1; gopkg.in/yaml.v3 implements a YAML-1.2-flavoured
// core. Left to themselves the two disagree about what a bare word means —
// `title: yes` is a boolean to PyYAML and a string to yaml.v3, `title: 1e3` is
// a string to PyYAML and a float to yaml.v3 — and a note would then pass or
// fail depending on which checker happened to be installed. That is the one
// failure the two-validator design cannot absorb, so plain scalars are resolved
// here exactly the way PyYAML's SafeLoader resolves them. The regexps below are
// transcribed from PyYAML's resolver.py and tried in PyYAML's own order.
//
// Quoted, literal and folded scalars never reach this: a quoted value is a
// string in both implementations, always.
//
// Two deliberate departures from PyYAML, both matched on the Python side:
//
//   - Timestamps are NOT resolved; a date keeps the text its author wrote.
//     PyYAML reads a bare `2026-01-31` as a date but leaves `2026-1-5` a
//     string, while yaml.v3 reads both as time.Time — so reformatting a parsed
//     date silently repairs input the other side rejects, and `created:
//     2026-1-5` passes here while failing there. The vault's loader has the
//     timestamp resolver removed for exactly this reason; see
//     scripts/_vault.py.
//   - "=" resolves to an error rather than to YAML 1.1's !!value tag, because
//     PyYAML's SafeConstructor has no constructor for that tag and raises.

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Transcribed from PyYAML resolver.py. Tried in the order PyYAML adds them:
// bool, float, int, null, value. The sets are disjoint, so the order is
// documentation rather than tie-breaking — but it is PyYAML's order, so a
// future edit that makes two of them overlap resolves the way PyYAML would.
var (
	plainBool = regexp.MustCompile(`^(?:yes|Yes|YES|no|No|NO|` +
		`true|True|TRUE|false|False|FALSE|` +
		`on|On|ON|off|Off|OFF)$`)

	plainFloat = regexp.MustCompile(`^(?:` +
		`[-+]?(?:[0-9][0-9_]*)\.[0-9_]*(?:[eE][-+][0-9]+)?` +
		`|\.[0-9_]+(?:[eE][-+][0-9]+)?` +
		`|[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+\.[0-9_]*` +
		`|[-+]?\.(?:inf|Inf|INF)` +
		`|\.(?:nan|NaN|NAN))$`)

	plainInt = regexp.MustCompile(`^(?:` +
		`[-+]?0b[0-1_]+` +
		`|[-+]?0[0-7_]+` +
		`|[-+]?(?:0|[1-9][0-9_]*)` +
		`|[-+]?0x[0-9a-fA-F_]+` +
		`|[-+]?[1-9][0-9_]*(?::[0-5]?[0-9])+)$`)

	plainNull = regexp.MustCompile(`^(?:~|null|Null|NULL|)$`)
)

// resolvePlain returns the value PyYAML's SafeLoader would construct for a
// plain scalar. The error case mirrors a PyYAML ConstructorError, which the
// Python validator surfaces as "frontmatter is not valid YAML".
func resolvePlain(raw string) (interface{}, error) {
	switch {
	case plainBool.MatchString(raw):
		return constructBool(raw), nil
	case plainFloat.MatchString(raw):
		return constructFloat(raw), nil
	case plainInt.MatchString(raw):
		return constructInt(raw), nil
	case plainNull.MatchString(raw):
		return nil, nil
	case raw == "=":
		// PyYAML resolves this to tag:yaml.org,2002:value, for which
		// SafeConstructor defines nothing, and raises.
		return nil, fmt.Errorf("could not determine a constructor for the tag "+
			"'tag:yaml.org,2002:value' (the bare %q scalar)", raw)
	}
	return raw, nil
}

func constructBool(raw string) bool {
	switch strings.ToLower(raw) {
	case "yes", "true", "on":
		return true
	}
	return false
}

// sign strips a leading + or - and reports the multiplier, as PyYAML's
// constructors do before parsing the digits.
func sign(s string) (string, int64) {
	if strings.HasPrefix(s, "-") {
		return s[1:], -1
	}
	return strings.TrimPrefix(s, "+"), 1
}

// constructInt mirrors PyYAML's construct_yaml_int, including YAML 1.1's
// leading-zero octal and base-60 sexagesimal forms. A value too large for
// int64 falls back to a float, where Python would keep an exact bignum — the
// difference is unreachable through any schema keyword this vault uses, and a
// wrong-by-rounding number still fails every check an out-of-range one does.
func constructInt(raw string) interface{} {
	s := strings.ReplaceAll(raw, "_", "")
	s, mul := sign(s)

	var (
		v   int64
		err error
	)
	switch {
	case strings.HasPrefix(s, "0b"):
		v, err = strconv.ParseInt(s[2:], 2, 64)
	case strings.HasPrefix(s, "0x"):
		v, err = strconv.ParseInt(s[2:], 16, 64)
	case s != "0" && strings.HasPrefix(s, "0"):
		v, err = strconv.ParseInt(s, 8, 64)
	case strings.Contains(s, ":"):
		for _, part := range strings.Split(s, ":") {
			d, perr := strconv.ParseInt(part, 10, 64)
			if perr != nil {
				err = perr
				break
			}
			v = v*60 + d
		}
	default:
		v, err = strconv.ParseInt(s, 10, 64)
	}
	if err != nil {
		if f, ferr := strconv.ParseFloat(s, 64); ferr == nil {
			return float64(mul) * f
		}
		return raw // unparseable after all: leave the text alone
	}
	return mul * v
}

// constructFloat mirrors PyYAML's construct_yaml_float, including .inf/.nan
// and the base-60 sexagesimal form.
func constructFloat(raw string) float64 {
	s := strings.ToLower(strings.ReplaceAll(raw, "_", ""))
	s, mul := sign(s)

	switch s {
	case ".inf":
		return math.Inf(int(mul))
	case ".nan":
		return math.NaN()
	}
	if strings.Contains(s, ":") {
		var v float64
		for _, part := range strings.Split(s, ":") {
			d, err := strconv.ParseFloat(part, 64)
			if err != nil {
				return 0
			}
			v = v*60 + d
		}
		return float64(mul) * v
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return float64(mul) * f
}

// pythonStr renders a resolved scalar the way Python's str() would, for use as
// a mapping key. JSON objects are keyed by strings and YAML mappings are not,
// so both implementations have to agree on how a non-string key is spelled;
// the Python side reaches this through str(k) in its normalise().
func pythonStr(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case string:
		return x
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		switch {
		case math.IsInf(x, 1):
			return "inf"
		case math.IsInf(x, -1):
			return "-inf"
		case math.IsNaN(x):
			return "nan"
		case x == math.Trunc(x) && math.Abs(x) < 1e16:
			// Python renders an integral float with a trailing ".0".
			return strconv.FormatFloat(x, 'f', 1, 64)
		}
		return strconv.FormatFloat(x, 'g', -1, 64)
	}
	return fmt.Sprintf("%v", v)
}
