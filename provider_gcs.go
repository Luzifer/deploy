package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"sort"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

func init() { registerStorageProvider(&storageGCS{}) }

type storageGCS struct {
	bucket *storage.BucketHandle
	client *storage.Client
	prefix string
}

// InitializeFromURI retrieves the user input URI and must decide whether
// it can initialize from that or can't. If the URI is not suitable for the
// provider an errInitializationNotPossible error needs to be returned. If
// the initialization failed because of an error it must be returned.
func (s *storageGCS) InitializeFromURI(uri string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return err
	}

	if u.Scheme != "gs" {
		return errInitializationNotPossible
	}

	s.prefix = u.Path

	s.client, err = storage.NewClient(context.Background())
	if err != nil {
		return err
	}

	s.bucket = s.client.Bucket(u.Host)
	return nil
}

// GetLatestDeployment retrieves a software identifier and must return the
// latest deployment ID for this software. In case a the identifier does not
// exist an errNoDeploymentFound error must be returned.
func (s storageGCS) GetLatestDeployment(identifier string) (string, error) {
	deployments := []*storage.ObjectAttrs{}

	it := s.bucket.Objects(context.Background(), &storage.Query{
		Prefix: s.prefix,
	})

	for {
		attr, err := it.Next()
		if err == iterator.Done {
			break
		}

		if err != nil {
			return "", err
		}

		if strings.HasPrefix(attr.Name, identifier) && strings.HasSuffix(attr.Name, ".zip") {
			deployments = append(deployments, attr)
		}
	}

	if len(deployments) == 0 {
		return "", errNoDeploymentFound
	}

	sort.Slice(deployments, func(i, j int) bool {
		return deployments[i].Updated.Before(deployments[j].Updated)
	})

	deploymentID := deployments[len(deployments)-1].Name
	deploymentID = strings.TrimPrefix(deploymentID, identifier)
	deploymentID = strings.TrimSuffix(deploymentID, ".zip")
	return deploymentID, nil
}

// GetDeploymentArtifact retrieves an software identifier and a deployment
// ID and must return an io.ReaderAt containing the ZIP-file of the artifact
// and the size of the ZIP-file. In case there is no artifact for the given
// identifier and deploymentID an errNoSuchDeployment error must be returned.
func (s storageGCS) GetDeploymentArtifact(identifier, deploymentID string) (io.ReaderAt, int64, error) {
	obj := s.bucket.Object(path.Join(s.prefix, identifier+deploymentID+".zip"))
	r, err := obj.NewReader(context.Background())
	if err != nil {
		return nil, 0, err
	}
	defer r.Close()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, r); err != nil {
		return nil, 0, err
	}

	return bytes.NewReader(buf.Bytes()), int64(buf.Len()), nil
}

// String must return a string representation of the provider for debug logging
func (s storageGCS) String() string {
	return fmt.Sprintf("GCE provider at bucket %q with prefix %q", s.bucket, s.prefix)
}
