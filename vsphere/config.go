package vsphere

import (
	"context"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"
	"net/url"
)

func NewClient(ctx context.Context, vSphereHost, vSphereUsername, vSpherePassword string) (*govmomi.Client, error) {

	u, err := soap.ParseURL(vSphereHost)
	if err != nil {
		return nil, err
	}

	u.User = url.UserPassword(vSphereUsername, vSpherePassword)

	return govmomi.NewClient(ctx, u, true)
}