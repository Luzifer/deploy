package main

import (
	"archive/zip"
	"fmt"
	"os"

	"github.com/Luzifer/rconfig"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
)

var (
	cfg = struct {
		FetchCron          string `flag:"fetch-cron,c" default:"* * * * *" description:"When to query for new deployments (cron syntax)"`
		LogLevel           string `flag:"log-level" default:"info" description:"Log level (debug, info, warn, error, fatal)"`
		SoftwareIdentifier string `flag:"identifier,i" default:"default" description:"Software identifier to query deployments for"`
		StorageURI         string `flag:"storage,s" default:"" description:"URI for the storage provider to use" validate:"nonzero"`
		VersionAndExit     bool   `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	version = "dev"
)

func init() {
	if err := rconfig.ParseAndValidate(&cfg); err != nil {
		log.Fatalf("Unable to parse commandline options: %s", err)
	}

	if cfg.VersionAndExit {
		fmt.Printf("deploy %s\n", version)
		os.Exit(0)
	}

	if l, err := log.ParseLevel(cfg.LogLevel); err != nil {
		log.WithError(err).Fatal("Unable to parse log level")
	} else {
		log.SetLevel(l)
	}
}

func main() {
	var lastDeployed string

	storage, err := getConfiguredStorageProvider(cfg.StorageURI)
	if err != nil {
		log.WithError(err).Fatal("Unable to open storage")
	}

	log.WithFields(log.Fields{
		"provider": storage.String(),
	}).Debug("Storage initialized")

	actChan := make(chan struct{}, 1)
	actChan <- struct{}{}

	c := cron.New()
	c.AddFunc("0 "+cfg.FetchCron, func() {
		actChan <- struct{}{}
	})
	c.Start()

	for range actChan {
		log.Debug("Start fetching latest deployment")
		deployment, err := storage.GetLatestDeployment(cfg.SoftwareIdentifier)
		if err != nil {
			log.WithError(err).Error("Unable to get latest deployment ID")
			continue
		}

		logger := log.WithFields(log.Fields{
			"deployment_id": deployment,
		})

		if deployment == lastDeployed {
			logger.Debug("Latest deployment already deployed")
			continue
		}

		logger.Debug("Starting deployment")

		if err := executeDeployment(storage, deployment); err != nil {
			logger.WithError(err).Error("Deployment failed")
			continue
		}
		lastDeployed = deployment

		logger.Info("Deployment succeeded")
	}
}

func executeDeployment(storage storageProvider, deploymentIdentifer string) error {
	deployZipRaw, size, err := storage.GetDeploymentArtifact(cfg.SoftwareIdentifier, deploymentIdentifer)
	if err != nil {
		return fmt.Errorf("Unable to fetch deployment ZIP: %s", err)
	}

	zipFile, err := zip.NewReader(deployZipRaw, size)
	if err != nil {
		return fmt.Errorf("Unable to read deployment ZIP: %s", err)
	}

	as, err := parseZIPAppSpec(zipFile)
	if err != nil {
		return err
	}

	return as.Execute(zipFile)
}
