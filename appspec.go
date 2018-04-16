package main

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

type appspecFile struct {
	Source      string `file:"source"`
	Destination string `file:"destination"`
}

func (a appspecFile) copyFile(f *zip.File, stripPrefix string) error {
	targetFile := path.Join(a.Destination, strings.TrimPrefix(f.Name, stripPrefix))

	if err := os.MkdirAll(path.Dir(targetFile), 0755); err != nil {
		return fmt.Errorf("Unable to create destination directory %q: %s", a.Destination, err)
	}

	fp, err := os.OpenFile(targetFile, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, f.Mode())
	if err != nil {
		return fmt.Errorf("Unable to open destination %q for writing: %s", targetFile, err)
	}
	defer fp.Close()

	zfp, err := f.Open()
	if err != nil {
		return fmt.Errorf("Unable to read source %q from ZIP file: %s", f.Name, err)
	}
	defer zfp.Close()

	_, err = io.Copy(fp, zfp)
	return err
}

func (a appspecFile) Execute(zipFile *zip.Reader) error {
	// https://docs.aws.amazon.com/codedeploy/latest/userguide/reference-appspec-file-structure-files.html
	//
	// - If source refers to a file, only the specified files are copied to the instance.
	// - If source refers to a directory, then all files in the directory are copied to the instance.
	// - If source is a single slash ("/" for Amazon Linux, RHEL, and Ubuntu Server instances, or "\"
	//   for Windows Server instances), then all of the files from your revision are copied to the instance.
	//
	// Note: This tool does not have Windows support!

	for _, f := range zipFile.File {
		if f.Name == a.Source {
			// Exact match (case 1)
			return a.copyFile(f, path.Dir(f.Name))
		}
	}

	// No exact match, fall back to prefix matching
	if a.Source == "/" {
		// For prefix matching ignore the slash which will match everything (case 3)
		a.Source = ""
	}

	for _, f := range zipFile.File {
		if strings.HasPrefix(f.Name, a.Source) && !f.FileInfo().IsDir() {
			if err := a.copyFile(f, a.Source); err != nil {
				return err
			}
		}
	}

	return nil
}

type appspecHook struct {
	Location string `yaml:"location"`
	Timeout  int    `yaml:"timeout"`
	RunAs    string `yaml:"runas"`
}

func (a appspecHook) Execute(zipFile *zip.Reader, logger *log.Entry) error {
	var (
		err    error
		script io.ReadCloser
	)

	for _, f := range zipFile.File {
		if f.Name == a.Location {
			script, err = f.Open()
			if err != nil {
				return fmt.Errorf("Unable to open script %q from ZIP file: %s", a.Location, err)
			}
			defer script.Close()
			break
		}
	}

	if script == nil {
		return fmt.Errorf("Script %q not found in ZIP file", a.Location)
	}

	if a.Timeout == 0 {
		a.Timeout = 3600
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.Timeout)*time.Second)
	defer cancel()

	stdout := logger.WriterLevel(log.InfoLevel)
	defer stdout.Close()
	stderr := logger.WriterLevel(log.ErrorLevel)
	defer stderr.Close()

	cmd := exec.CommandContext(ctx, "/bin/bash")
	cmd.Stdin = script
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// OS specific function, see in appspec_GOOS.go files
	if err := a.setRunAs(cmd); err != nil {
		return fmt.Errorf("Unable to set RunAs user: %s", err)
	}

	return cmd.Run()
}

type appspec struct {
	Version float64 `yaml:"version"`
	// OS ignored
	Files []appspecFile `yaml:"files"`
	// Permissions ignored
	Hooks map[string][]appspecHook `yaml:"hooks"`
}

func parseZIPAppSpec(zipFile *zip.Reader) (*appspec, error) {
	for _, f := range zipFile.File {
		if f.Name == "appspec.yml" {
			fr, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer fr.Close()

			as := &appspec{}
			return as, yaml.NewDecoder(fr).Decode(as)
		}
	}

	return nil, errors.New("appspec.yml not found in ZIP file")
}

// Execute runs the directives specified inside the appspec definition
func (a appspec) Execute(zipFile *zip.Reader, logger *log.Entry) error {
	if err := a.Validate(); err != nil {
		return err
	}

	// Flow definition
	// https://docs.aws.amazon.com/codedeploy/latest/userguide/reference-appspec-file-structure-hooks.html
	// [Start] => [DownloadBundle] => BeforeInstall => [Install] => AfterInstall => ApplicationStart => ValidateService => [End]
	// Unsupported: ApplicationStop, tasks need to be moved to BeforeInstall
	// [] = System tasks, all others are definable by users

	if hooks, ok := a.Hooks["BeforeInstall"]; ok {
		for _, hook := range hooks {
			if err := hook.Execute(zipFile, logger); err != nil {
				return fmt.Errorf("Hook \"BeforeInstall\" failed: %s", err)
			}
		}
	}

	// Install
	for _, af := range a.Files {
		if err := af.Execute(zipFile); err != nil {
			return fmt.Errorf("File operation failed: %s", err)
		}
	}

	for _, hookName := range []string{"AfterInstall", "ApplicationStart", "ValidateService"} {
		if hooks, ok := a.Hooks[hookName]; ok {
			for _, hook := range hooks {
				if err := hook.Execute(zipFile, logger); err != nil {
					return fmt.Errorf("Hook %q failed: %s", hookName, err)
				}
			}
		}
	}

	return nil
}

// Validate executes some basic tests on the parsed appspec definition
func (a appspec) Validate() error {
	if a.Version != 0.0 {
		return errors.New("Unsupported appspec version")
	}

	return nil
}
