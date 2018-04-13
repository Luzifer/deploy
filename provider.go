package main

import (
	"errors"
	"io"
	"sync"
)

type storageProvider interface {
	// InitializeFromURI retrieves the user input URI and must decide whether
	// it can initialize from that or can't. If the URI is not suitable for the
	// provider an errInitializationNotPossible error needs to be returned. If
	// the initialization failed because of an error it must be returned.
	InitializeFromURI(uri string) error
	// GetLatestDeployment retrieves a software identifier and must return the
	// latest deployment ID for this software. In case a the identifier does not
	// exist an errNoDeploymentFound error must be returned.
	GetLatestDeployment(identifier string) (string, error)
	// GetDeploymentArtifact retrieves an software identifier and a deployment
	// ID and must return an io.ReaderAt containing the ZIP-file of the artifact
	// and the size of the ZIP-file. In case there is no artifact for the given
	// identifier and deploymentID an errNoSuchDeployment error must be returned.
	GetDeploymentArtifact(identifier, deploymentID string) (io.ReaderAt, int64, error)
	// String must return a string representation of the provider for debug logging
	String() string
}

var (
	errInitializationNotPossible = errors.New("Initialization not possible from given URI")
	errNoDeploymentFound         = errors.New("No deployment was found for the given identifier")
	errNoSuchDeployment          = errors.New("The given deployment id was not found for the identifier")

	storageProviders    []storageProvider
	storageProviderLock sync.Mutex
)

func registerStorageProvider(s storageProvider) {
	storageProviderLock.Lock()
	defer storageProviderLock.Unlock()

	storageProviders = append(storageProviders, s)
}

func getConfiguredStorageProvider(uri string) (storageProvider, error) {
	storageProviderLock.Lock()
	defer storageProviderLock.Unlock()

	for _, sp := range storageProviders {
		if err := sp.InitializeFromURI(uri); err == nil {
			return sp, nil
		}
	}

	return nil, errInitializationNotPossible
}
