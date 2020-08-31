package vsphere

import (
	"context"
	"fmt"
	"github.com/roshankarande/go-vsphere/vsphere/guest/toolbox"
	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"io"
)

func InvokeCommands(ctx context.Context,auth types.BaseGuestAuthentication, opsmgr *guest.OperationsManager, data chan string, commands []string) error {

	pmgr, err := opsmgr.ProcessManager(ctx)

	if err != nil {
		return err
	}

	fmgr, err := opsmgr.FileManager(ctx)

	if err != nil {
		return err
	}

	tboxClient := &toolbox.Client{
		ProcessManager: pmgr,
		FileManager:    fmgr,
		Authentication: auth,
		GuestFamily:    types.VirtualMachineGuestOsFamilyWindowsGuest,
	}

	err = tboxClient.RunCommands(ctx,data,commands)

	if err != nil {
		return err
	}

	return nil
}

func InvokeScript(ctx context.Context,auth types.BaseGuestAuthentication, opsmgr *guest.OperationsManager,data chan string, script string) error {

	pmgr, err := opsmgr.ProcessManager(ctx)

	if err != nil {
		return err
	}

	fmgr, err := opsmgr.FileManager(ctx)

	if err != nil {
		return err
	}

	tboxClient := &toolbox.Client{
		ProcessManager: pmgr,
		FileManager:    fmgr,
		Authentication: auth,
		GuestFamily:    types.VirtualMachineGuestOsFamilyWindowsGuest,
	}

	err = tboxClient.RunScript(ctx,data,script)

	if err != nil {
		return err
	}

	return nil
}

func TestCredentials(ctx context.Context, baseGuestAuth types.BaseGuestAuthentication, opsmgr *guest.OperationsManager) error {

	authmgr, err := opsmgr.AuthManager(ctx)

	if err != nil {
		return err
	}

	err = authmgr.ValidateCredentials(ctx, baseGuestAuth)

	if err != nil {
		return err
	}

	return nil
}

func Upload(ctx context.Context, auth types.BaseGuestAuthentication, opsmgr *guest.OperationsManager, f io.Reader, suffix, dst string, isDir bool) error {

	pmgr, err := opsmgr.ProcessManager(ctx)

	if err != nil {
		return err
	}

	fmgr, err := opsmgr.FileManager(ctx)

	if err != nil {
		return err
	}

	c := &toolbox.Client{
		ProcessManager: pmgr,
		FileManager:    fmgr,
		Authentication: auth,
		GuestFamily:    types.VirtualMachineGuestOsFamilyWindowsGuest,
	}

	vcFile, err := c.FileManager.CreateTemporaryFile(ctx, c.Authentication, "", suffix, "")

	if err != nil {
		return err
	}

	defer c.FileManager.DeleteFile(ctx, c.Authentication, vcFile)

	p := soap.DefaultUpload
	err = c.Upload(ctx, f, vcFile, p, &types.GuestFileAttributes{},true)
	if err != nil {
		return err
	}

	if isDir {
		cmd := fmt.Sprintf("tar -xzvf %s -C %s", vcFile, dst)
		c.RunSimpleCommands(ctx, []string{fmt.Sprintf("mkdir %s -Force",dst),cmd})
	} else{
		err = c.FileManager.MoveFile(ctx, c.Authentication, vcFile, dst,true)
		if err != nil {
			return err
		}
	}

	return nil

}


