package format

import (
	"encoding/xml"
	"fmt"
	"io"
)

// JUnit emits diagnostics as JUnit XML. The schema is the same shape
// CI dashboards (Jenkins, CircleCI, GitLab, Azure, Buildkite, etc.)
// already parse for test results, which lets a lint failure show up
// alongside test failures with no additional integration.
//
// Each diagnostic becomes a <testcase> with a <failure> child. The
// rule id is the testcase's classname so dashboards can group "all
// no-floating-promises failures" naturally.
type JUnit struct{}

// Name returns "junit".
func (JUnit) Name() string { return "junit" }

type junitTestsuites struct {
	XMLName  xml.Name        `xml:"testsuites"`
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Errors   int             `xml:"errors,attr"`
	Suites   []junitTestsuite `xml:"testsuite"`
}

type junitTestsuite struct {
	Name     string         `xml:"name,attr"`
	Tests    int            `xml:"tests,attr"`
	Failures int            `xml:"failures,attr"`
	Errors   int            `xml:"errors,attr"`
	Cases    []junitTestcase `xml:"testcase"`
}

type junitTestcase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Body    string `xml:",chardata"`
}

// Format writes a JUnit XML document. The single <testsuite> matches
// what most CI dashboards expect; nested suites would require us to
// pick a grouping (by file? by rule?) and CI dashboards generally
// flatten anyway. Output is sorted and the encoder is deterministic
// so byte-identical output holds for byte-identical input.
func (JUnit) Format(w io.Writer, diagnostics []Diagnostic) error {
	SortDiagnostics(diagnostics)

	cases := make([]junitTestcase, 0, len(diagnostics))
	for _, d := range diagnostics {
		// testcase name is the most-specific identifier; classname is
		// the rule so CI grouping by classname produces "by rule".
		cases = append(cases, junitTestcase{
			Name:      fmt.Sprintf("%s:%d:%d", d.Range.File, d.Range.StartLine, d.Range.StartColumn),
			Classname: d.RuleID,
			Failure: &junitFailure{
				Message: d.Message,
				Type:    d.RuleID,
				Body:    fmt.Sprintf("%s:%d:%d %s %s", d.Range.File, d.Range.StartLine, d.Range.StartColumn, d.RuleID, d.Message),
			},
		})
	}

	doc := junitTestsuites{
		Name:     "jetlint",
		Tests:    len(diagnostics),
		Failures: len(diagnostics),
		Suites: []junitTestsuite{{
			Name:     "jetlint",
			Tests:    len(diagnostics),
			Failures: len(diagnostics),
			Cases:    cases,
		}},
	}

	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	return nil
}
