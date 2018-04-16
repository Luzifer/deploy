package main

import "sync"

var (
	reporters     []reporter
	reportersLock sync.Mutex
)

type reporterList []reporter

func (r reporterList) Execute(success bool, content []byte) error {
	for _, i := range r {
		if err := i.Execute(success, content); err != nil {
			return err
		}
	}

	return nil
}

type reporter interface {
	// InitializeFromURI retrieves the user input URI and must decide whether
	// it can initialize from that or can't. If the URI is not suitable for the
	// provider an errInitializationNotPossible error needs to be returned. If
	// the initialization failed because of an error it must be returned.
	InitializeFromURI(uri string) error
	// Execute takes the content of the reporting and executes the
	// delivery of the message to the specified targets.
	Execute(success bool, content []byte) error
}

func registerReporter(r reporter) {
	reportersLock.Lock()
	defer reportersLock.Unlock()

	reporters = append(reporters, r)
}

func initializeReporters(uris []string) (reporterList, error) {
	reportersLock.Lock()
	defer reportersLock.Unlock()

	rs := reporterList{}

	for _, uri := range uris {
		for _, r := range reporters {
			if err := r.InitializeFromURI(uri); err != nil {
				if err == errInitializationNotPossible {
					continue
				}
				return nil, err
			}
			rs = append(rs, r)
		}
	}

	return rs, nil
}
