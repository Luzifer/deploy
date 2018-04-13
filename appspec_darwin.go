// +build !linux

package main

import "os/exec"

func (a appspecHook) setRunAs(cmd *exec.Cmd) error { return nil }
