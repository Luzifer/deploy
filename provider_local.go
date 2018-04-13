package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
)

func init() {
	registerStorageProvider(&storageLocal{})
}

type storageLocal struct {
	path string
}

// InitializeFromURI retrieves the user input URI and must decide whether
// it can initialize from that or can't. If the URI is not suitable for the
// provider an errInitializationNotPossible error needs to be returned. If
// the initialization failed because of an error it must be returned.
func (s *storageLocal) InitializeFromURI(uri string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return err
	}

	if u.Scheme != "file" {
		return errInitializationNotPossible
	}

	s.path = u.Path
	return nil
}

// GetLatestDeployment retrieves a software identifier and must return the
// latest deployment ID for this software. In case a the identifier does not
// exist an errNoDeploymentFound error must be returned.
func (s storageLocal) GetLatestDeployment(identifier string) (string, error) {
	files, err := ioutil.ReadDir(s.path)
	if err != nil {
		return "", err
	}

	deployments := []os.FileInfo{}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		if !strings.HasPrefix(f.Name(), identifier) || !strings.HasSuffix(f.Name(), ".zip") {
			continue
		}

		deployments = append(deployments, f)
	}

	if len(deployments) == 0 {
		return "", errNoDeploymentFound
	}

	sort.Slice(deployments, func(i, j int) bool {
		return deployments[i].ModTime().Before(deployments[j].ModTime())
	})

	lastDeployment := deployments[len(deployments)-1].Name()
	lastDeployment = strings.TrimSuffix(lastDeployment, ".zip")
	lastDeployment = strings.TrimPrefix(lastDeployment, identifier)

	return lastDeployment, nil
}

// GetDeploymentArtifact retrieves an software identifier and a deployment
// ID and must return an io.ReaderAt containing the ZIP-file of the artifact
// and the size of the ZIP-file. In case there is no artifact for the given
// identifier and deploymentID an errNoSuchDeployment error must be returned.
func (s storageLocal) GetDeploymentArtifact(identifier, deploymentID string) (io.ReaderAt, int64, error) {
	rawZip, err := ioutil.ReadFile(path.Join(s.path, identifier+deploymentID+".zip"))
	if err != nil {
		return nil, 0, err
	}

	return bytes.NewReader(rawZip), int64(len(rawZip)), nil
}

// String must return a string representation of the provider for debug logging
func (s storageLocal) String() string {
	return fmt.Sprintf("Local file provider at %q", s.path)
}
