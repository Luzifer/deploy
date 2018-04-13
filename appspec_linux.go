package main

import (
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

func (a appspecHook) setRunAs(cmd *exec.Cmd) error {
	if a.RunAs == "" {
		return nil
	}

	usr, err := user.Lookup(a.RunAs)
	if err != nil {
		return fmt.Errorf("Unable to find UID for user %q: %s", a.RunAs, err)
	}

	uid, err := strconv.ParseInt(usr.Uid, 10, 64)
	if err != nil {
		return fmt.Errorf("User %q had no numeric UID: %s", a.RunAs, err)
	}
	gid, err := strconv.ParseInt(usr.Gid, 10, 64)
	if err != nil {
		return fmt.Errorf("User %q had no numeric GID: %s", a.RunAs, err)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}

	return nil
}
