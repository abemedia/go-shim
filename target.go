package shim

import (
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
)

// A Target is a build a project publishes: the names it goes by and what it needs to run.
//
// Every field other than Names is a Go environment variable and takes one of its documented
// values, read as Go reads it, so GOARM "7" means "7,hardfloat" and "5" means "5,softfloat". A set
// field is the least the build needs, so an entry with GOARM "6" also serves an ARMv7 host. An
// empty field is no requirement, so an entry with neither GOOS nor GOARCH is a build that runs
// anywhere.
type Target struct {
	GOOS   string
	GOARCH string

	// Names are the .Target values to try for this build, in order.
	Names []string

	GO386     string
	GOAMD64   string
	GOARM     string
	GOARM64   string
	GOMIPS    string
	GOMIPS64  string
	GOPPC64   string
	GORISCV64 string
}

func host(build *debug.BuildInfo) Target {
	h := Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	for _, s := range build.Settings {
		switch s.Key {
		case "GO386":
			h.GO386 = s.Value
		case "GOAMD64":
			h.GOAMD64 = s.Value
		case "GOARM":
			h.GOARM = s.Value
		case "GOARM64":
			h.GOARM64 = s.Value
		case "GOMIPS":
			h.GOMIPS = s.Value
		case "GOMIPS64":
			h.GOMIPS64 = s.Value
		case "GOPPC64":
			h.GOPPC64 = s.Value
		case "GORISCV64":
			h.GORISCV64 = s.Value
		}
	}
	return h
}

func (t Target) matches(h Target) bool {
	return (t.GOOS == "" || t.GOOS == h.GOOS) &&
		(t.GOARCH == "" || t.GOARCH == h.GOARCH) &&
		(t.GO386 == "" || go386(t.GO386, h.GO386)) &&
		(t.GOAMD64 == "" || goamd64(t.GOAMD64, h.GOAMD64)) &&
		(t.GOARM == "" || goarm(t.GOARM, h.GOARM)) &&
		(t.GOARM64 == "" || goarm64(t.GOARM64, h.GOARM64)) &&
		(t.GOMIPS == "" || gomips(t.GOMIPS, h.GOMIPS)) &&
		(t.GOMIPS64 == "" || gomips(t.GOMIPS64, h.GOMIPS64)) &&
		(t.GOPPC64 == "" || goppc64(t.GOPPC64, h.GOPPC64)) &&
		(t.GORISCV64 == "" || goriscv64(t.GORISCV64, h.GORISCV64))
}

func go386(want, have string) bool {
	switch want {
	case "softfloat":
		return true
	case "sse2":
		return have == "sse2"
	}
	return false
}

func goamd64(want, have string) bool {
	return atLeast(want, have, "v")
}

func goarm(want, have string) bool {
	soft := func(v string) bool { return v == "5" || strings.HasSuffix(v, ",softfloat") }
	w := number(want, "")
	return w >= 5 && number(have, "") >= w && (soft(want) || !soft(have))
}

func goarm64(want, have string) bool {
	w, wext, _ := strings.Cut(want, ",")
	h, _, _ := strings.Cut(have, ",")
	wv, err := strconv.ParseFloat(strings.TrimPrefix(w, "v"), 64)
	hv, _ := strconv.ParseFloat(strings.TrimPrefix(h, "v"), 64)
	if hv >= 9 && wv < 9 {
		// ARMv9.0 was cut from ARMv8.5, so a v9 host covers v8 only five minors further on.
		hv -= 0.5
	}
	if err != nil || hv < wv {
		return false
	}
	for _, ext := range strings.Split(wext, ",") {
		if ext != "" && !strings.Contains(have, ","+ext) && !(ext == "lse" && hv >= 8.1) {
			return false
		}
	}
	return true
}

func gomips(want, have string) bool {
	switch want {
	case "softfloat":
		return true
	case "hardfloat":
		return have == "hardfloat"
	}
	return false
}

func goppc64(want, have string) bool {
	return atLeast(want, have, "power")
}

func goriscv64(want, have string) bool {
	return atLeast(want, have, "rva")
}

func atLeast(want, have, prefix string) bool {
	w := number(want, prefix)
	return w >= 0 && number(have, prefix) >= w
}

func number(value, prefix string) int {
	value, ok := strings.CutPrefix(value, prefix)
	if !ok {
		return -1
	}
	end := 0
	for end < len(value) && '0' <= value[end] && value[end] <= '9' {
		end++
	}
	n, err := strconv.Atoi(value[:end])
	if err != nil {
		return -1
	}
	return n
}
