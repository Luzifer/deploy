package main

import (
	"archive/zip"
	"fmt"
	"os"

	"github.com/Luzifer/rconfig"
	"github.com/contentflow/deploy/bufferhook"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
)

var (
	cfg = struct {
		FetchCron          string   `flag:"fetch-cron,c" default:"* * * * *" description:"When to query for new deployments (cron syntax)"`
		LogLevel           string   `flag:"log-level" default:"info" description:"Log level (debug, info, warn, error, fatal)"`
		Reporters          []string `flag:"reporter,r" default:"" description:"Reporting URIs to notify about deployments"`
		SoftwareIdentifier string   `flag:"identifier,i" default:"default" description:"Software identifier to query deployments for"`
		StorageURI         string   `flag:"storage,s" default:"" description:"URI for the storage provider to use" validate:"nonzero"`
		VersionAndExit     bool     `flag:"version" default:"false" description:"Prints current version and exits"`

		logLevel log.Level
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
		cfg.logLevel = l
	}
}

func main() {
	var lastDeployed string

	storage, err := getConfiguredStorageProvider(cfg.StorageURI)
	if err != nil {
		log.WithError(err).Fatal("Unable to open storage")
	}

	reporting, err := initializeReporters(cfg.Reporters)
	if err != nil {
		log.WithError(err).Fatal("Unable to create reporters")
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
		buf := bufferhook.New(cfg.logLevel)
		actLog := log.New()
		actLog.SetLevel(cfg.logLevel)
		actLog.AddHook(buf)

		actLog.Debug("Start fetching latest deployment")
		deployment, err := storage.GetLatestDeployment(cfg.SoftwareIdentifier)
		if err != nil {
			actLog.WithError(err).Error("Unable to get latest deployment ID")
			continue
		}

		logger := actLog.WithFields(log.Fields{
			"deployment_id": deployment,
		})

		if deployment == lastDeployed {
			logger.Debug("Latest deployment already deployed")
			continue
		}

		logger.Info("Starting deployment")

		var success bool
		if err := executeDeployment(storage, deployment, logger); err != nil {
			logger.WithError(err).Error("Deployment failed")
		} else {
			lastDeployed = deployment
			logger.Info("Deployment succeeded")
			success = true
		}

		if err := reporting.Execute(success, buf.String(), deployment); err != nil {
			log.WithError(err).Error("Failed sending reports")
		}
	}
}

func executeDeployment(storage storageProvider, deploymentIdentifer string, logger *log.Entry) error {
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

	return as.Execute(zipFile, logger)
}
