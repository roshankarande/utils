package vsphere

import (
	"context"
	"fmt"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"reflect"
)

func GetVirtualMachines(ctx context.Context, c *vim25.Client, namepattern string) ([]mo.VirtualMachine, error) {
	// Create view of VirtualMachine objects
	m := view.NewManager(c)

	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, err
	}

	defer v.Destroy(ctx)

	// Retrieve summary property for all machines
	// Reference: http://pubs.vmware.com/vsphere-60/topic/com.vmware.wssdk.apiref.doc/vim.VirtualMachine.html
	var vms []mo.VirtualMachine

	//err = v.Retrieve(ctx, []string{"VirtualMachine"}, []string{"summary","guest.ipAddress","datastore","network"}, &vms)
	//err = v.RetrieveWithFilter(ctx, []string{"VirtualMachine"}, []string{"summary","guest.ipAddress","datastore","network"}, &vms,property.Filter{"name": namepattern})
	err = v.RetrieveWithFilter(ctx, []string{"VirtualMachine"}, []string{"summary", "guest", "datastore", "network", "runtime", "guestHeartbeatStatus", "storage", "config"}, &vms, property.Filter{"name": namepattern})

	if err != nil {
		return nil, err
	}

	//for _, vm := range vms {
	//	fmt.Printf("%s: %s\n", vm.Summary.Config.Name, vm.Summary.Config.GuestFullName)
	//	config := vm.Summary.Config
	//	fmt.Println(config.Name,config.Annotation,config.CpuReservation,config.NumCpu,config.GuestFullName,config.GuestId,config.InstanceUuid,config.MemoryReservation,config.MemorySizeMB,config.Uuid)
	//	//fmt.Println(vm.Config.CpuHotAddEnabled,vm.Config.CpuHotRemoveEnabled,vm.Config.CreateDate,vm.Config.Version,vm.Parent.Value)
	//	fmt.Println(vm.Guest.IpAddress,vm.Datastore,vm.Network)
	//	fmt.Println(vm.Storage.PerDatastoreUsage)
	//}

	return vms, nil
}

func GetVirtualMachineDevices(ctx context.Context, c *vim25.Client, name string) (object.VirtualDeviceList, error) {

	vm, err := find.NewFinder(c).VirtualMachine(ctx, name)

	if err != nil {
		return nil, err
	}

	var vms []mo.VirtualMachine

	vm.Properties(ctx,vm.Reference(),[]string{"summary", "guest"},&vms)

	var virtualDevicesFiltered []types.BaseVirtualDevice
	devices, _ := vm.Device(ctx)

	for _, d := range devices {
		switch d.(type) {
		case *types.VirtualDisk:
			virtualDevicesFiltered = append(virtualDevicesFiltered, d)
			disk := d.(*types.VirtualDisk)
			info := d.GetVirtualDevice().Backing.(*types.VirtualDiskFlatVer2BackingInfo)
			// info.EagerlyScrub
			fmt.Println("[*types.VirtualDisk]", info.Datastore, info.FileName, *info.ThinProvisioned, info.DiskMode, disk.Key, disk.ControllerKey, *disk.UnitNumber, disk.DeviceInfo.GetDescription().Summary, disk.DeviceInfo.GetDescription().Label, disk.CapacityInKB/(1024*1024))

		case *types.VirtualVmxnet3:
			virtualDevicesFiltered = append(virtualDevicesFiltered, d)
			vmxnet3Adapter := d.(*types.VirtualVmxnet3)
			fmt.Println("[*types.VirtualVmxnet3]", vmxnet3Adapter.Connectable.Connected, vmxnet3Adapter.Connectable.StartConnected, vmxnet3Adapter.MacAddress, *vmxnet3Adapter.UnitNumber, vmxnet3Adapter.ControllerKey)

		default:
			//fmt.Println("Neither Network Nor Disk")
			fmt.Printf("[%v]\n", reflect.TypeOf(d))

		}

	}

	return object.VirtualDeviceList(virtualDevicesFiltered), nil
}

type VMInfo struct {
	VirtualMachine mo.VirtualMachine
	DeviceList object.VirtualDeviceList
	Datastores []mo.Datastore
	Networks []mo.Network
	HostSystem mo.HostSystem
}

func GetVM(ctx context.Context, c *vim25.Client, name string) (*VMInfo, error)  {

	var vmInfo VMInfo

	vm, err := find.NewFinder(c).VirtualMachine(ctx, name)

	if err != nil {
		return nil, err
	}

	vm.Properties(ctx,vm.Reference(),[]string{"summary", "guest", "datastore", "network", "runtime", "guestHeartbeatStatus", "storage", "config"},&vmInfo.VirtualMachine)

	var virtualDevicesFiltered []types.BaseVirtualDevice
	devices, _ := vm.Device(ctx)

	for _, d := range devices {
		switch d.(type) {
		case *types.VirtualDisk:
			virtualDevicesFiltered = append(virtualDevicesFiltered, d)
			disk := d.(*types.VirtualDisk)
			info := d.GetVirtualDevice().Backing.(*types.VirtualDiskFlatVer2BackingInfo)
			// info.EagerlyScrub
			fmt.Println("[*types.VirtualDisk]", info.Datastore, info.FileName, *info.ThinProvisioned, info.DiskMode, disk.Key, disk.ControllerKey, *disk.UnitNumber, disk.DeviceInfo.GetDescription().Summary, disk.DeviceInfo.GetDescription().Label, disk.CapacityInKB/(1024*1024))

		case *types.VirtualVmxnet3:
			virtualDevicesFiltered = append(virtualDevicesFiltered, d)
			vmxnet3Adapter := d.(*types.VirtualVmxnet3)
			fmt.Println("[*types.VirtualVmxnet3]", vmxnet3Adapter.Connectable.Connected, vmxnet3Adapter.Connectable.StartConnected, vmxnet3Adapter.MacAddress, *vmxnet3Adapter.UnitNumber, vmxnet3Adapter.ControllerKey)

		default:
			//fmt.Println("Neither Network Nor Disk")
			fmt.Printf("[%v]\n", reflect.TypeOf(d))
		}
	}

	vmInfo.DeviceList = virtualDevicesFiltered

	pc := property.DefaultCollector(c)

	// see if you would want to check for Errors...
	pc.Retrieve(ctx,vmInfo.VirtualMachine.Datastore , []string{"name"}, &vmInfo.Datastores)
	pc.Retrieve(ctx,vmInfo.VirtualMachine.Network,[]string{"name"},&vmInfo.Networks)
	pc.Retrieve(ctx,[]types.ManagedObjectReference{*vmInfo.VirtualMachine.Runtime.Host},[]string{"name"},&vmInfo.HostSystem)

	return &vmInfo,nil

}
