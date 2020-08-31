package vsphere

import (
	"context"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

func GetHosts(ctx context.Context, c *vim25.Client, namepattern string) ([]mo.HostSystem, error) {

	m := view.NewManager(c)

	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		return nil, err
	}

	defer v.Destroy(ctx)

	var hostSystems []mo.HostSystem

	err = v.RetrieveWithFilter(ctx, []string{"HostSystem"}, []string{"runtime","summary","hardware","datastore","network","config"}, &hostSystems, property.Filter{"name": namepattern})

	if err != nil {
		return nil, err
	}

	return hostSystems, nil
}
