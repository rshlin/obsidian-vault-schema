package lint

import (
	"fmt"
	"os"
)

// Report collects errors and warnings across a check run, printed in the same
// "warn"/"FAIL"/"OK" vocabulary a vault's own in-tree check scripts use, so a
// `make check` line reads the same whichever tool emitted it.
type Report struct {
	Title    string
	Checked  int
	Errors   []finding
	Warnings []finding
}

type finding struct {
	Path    string
	Message string
}

func (r *Report) Errorf(path, format string, args ...interface{}) {
	r.Errors = append(r.Errors, finding{Path: path, Message: fmt.Sprintf(format, args...)})
}

func (r *Report) Warnf(path, format string, args ...interface{}) {
	r.Warnings = append(r.Warnings, finding{Path: path, Message: fmt.Sprintf(format, args...)})
}

// Finish prints every warning then every error to stdout and returns the
// process exit code: 1 if there were any errors, 0 otherwise.
func (r *Report) Finish() int {
	for _, f := range r.Warnings {
		fmt.Fprintf(os.Stdout, "  warn  %s: %s\n", f.Path, f.Message)
	}
	for _, f := range r.Errors {
		fmt.Fprintf(os.Stdout, "  FAIL  %s: %s\n", f.Path, f.Message)
	}
	summary := fmt.Sprintf("%s: %d checked, %d error(s), %d warning(s)", r.Title, r.Checked, len(r.Errors), len(r.Warnings))
	if len(r.Errors) > 0 {
		fmt.Fprintln(os.Stdout, summary)
		return 1
	}
	fmt.Fprintf(os.Stdout, "OK  %s\n", summary)
	return 0
}
