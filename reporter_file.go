package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

func init() { registerReporter(&reporterFile{}) }

type reporterFile struct {
	path string
}

// InitializeFromURI retrieves the user input URI and must decide whether
// it can initialize from that or can't. If the URI is not suitable for the
// provider an errInitializationNotPossible error needs to be returned. If
// the initialization failed because of an error it must be returned.
func (r *reporterFile) InitializeFromURI(uri string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return err
	}

	if u.Scheme != "file" {
		return errInitializationNotPossible
	}

	r.path = u.Path
	return nil
}

// Execute takes the content of the reporting and executes the
// delivery of the message to the specified targets.
func (r reporterFile) Execute(success bool, content, deploymentID, hostname string) error {
	fileName := r.path
	for k, v := range map[string]string{
		`{s}`: cfg.SoftwareIdentifier,
		`{i}`: deploymentID,
		`{h}`: hostname,
		`{t}`: time.Now().Format(`2006-01-02T15-04-05`),
		`{d}`: time.Now().Format(`2006-01-02`),
	} {
		fileName = strings.Replace(fileName, k, v, -1)
	}

	fp, err := os.OpenFile(fileName, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer fp.Close()

	var verb = "with failure"
	if success {
		verb = "successfully"
	}

	fmt.Fprintf(fp, "[%s] Deployment %q finished %s:\n", time.Now().Format(time.RFC3339), deploymentID, verb)
	fmt.Fprintln(fp, content)

	return nil
}
