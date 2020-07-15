package hypervisor

import (
	"fmt"
	v1stat "github.com/containerd/cgroups/stats/v1"
	zconfig "github.com/lf-edge/eve/api/go/config"
	"github.com/lf-edge/eve/pkg/pillar/types"
	uuid "github.com/satori/go.uuid"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
)

type KvmContainerMockImpl struct{}

func (k *KvmContainerMockImpl) InitContainerdClient() error {
	return nil
}

func (k *KvmContainerMockImpl) GetMetrics(ctrID string) (*v1stat.Metrics, error) {
	m := &v1stat.Metrics{}

	m.Memory = &v1stat.MemoryStat{}
	m.Memory.Usage = &v1stat.MemoryEntry{}
	m.Memory.Usage.Usage = 400000
	m.Memory.Usage.Max = 500000

	m.CPU = &v1stat.CPUStat{}
	m.CPU.Usage = &v1stat.CPUUsage{}
	m.CPU.Usage.Total = 80000000

	return m, nil
}

var hyperKvm Hypervisor
var kvmIntel, kvmArm kvmContext

func init() {
	var err error
	kvmContainerImpl = &KvmContainerMockImpl{}
	hyperKvm, err = GetHypervisor("kvm")
	if hyperKvm.Name() != "kvm" || err != nil {
		panic(fmt.Sprintf("Requested kvm hypervisor, got %s (with error %v) instead", hyperKvm.Name(), err))
	}

	// these ones are very much handcrafted just for the tests
	kvmIntel = kvmContext{
		domains:     map[string]int{},
		devicemodel: "pc-q35-3.1",
		dmExec:      "",
		dmArgs:      []string{},
	}
	kvmArm = kvmContext{
		domains:     map[string]int{},
		devicemodel: "virt",
		dmExec:      "",
		dmArgs:      []string{},
	}
}

func TestCreateDomConfigOnlyCom1(t *testing.T) {
	config := types.DomainConfig{
		UUIDandVersion: types.UUIDandVersion{UUID: uuid.NewV4(), Version: "1.0"},
		VmConfig: types.VmConfig{
			Kernel:     "/boot/kernel",
			Ramdisk:    "/boot/ramdisk",
			ExtraArgs:  "init=/bin/sh",
			Memory:     1024 * 1024 * 10,
			VCpus:      2,
			VncDisplay: 5,
			VncPasswd:  "rosebud",
		},
		VifList: []types.VifInfo{
			{Bridge: "bn0", Mac: "6a:00:03:61:a6:90", Vif: "nbu1x1"},
			{Bridge: "bn0", Mac: "6a:00:03:61:a6:91", Vif: "nbu1x2"},
		},
		IoAdapterList: []types.IoAdapter{
			{Type: types.IoCom, Name: "COM1"},
		},
	}
	disks := []types.DiskStatus{
		{Format: zconfig.Format_QCOW2, FileLocation: "/foo/bar.qcow2", Devtype: "HDD"},
		{Format: zconfig.Format_CONTAINER, FileLocation: "/foo/container", Devtype: "Unclassified"},
		{Format: zconfig.Format_RAW, FileLocation: "/foo/bar.raw", Devtype: "HDD_EMPTY"},
		{Format: zconfig.Format_RAW, FileLocation: "/foo/cd.iso", Devtype: "CDROM"},
	}
	aa := types.AssignableAdapters{
		Initialized: true,
		IoBundleList: []types.IoBundle{
			{
				Type:            types.IoCom,
				AssignmentGroup: "COM1",
				Phylabel:        "COM1",
				Ifname:          "COM1",
				Serial:          "/dev/ttyS0",
				UsedByUUID:      config.UUIDandVersion.UUID,
			},
		},
	}
	conf, err := ioutil.TempFile("/tmp", "config")
	if err != nil {
		t.Errorf("Can't create config file for a domain %v", err)
	} else {
		defer os.Remove(conf.Name())
	}

	t.Run("amd64", func(t *testing.T) {
		conf.Seek(0, 0)
		if err := kvmIntel.CreateDomConfig("test", config, disks, &aa, conf); err != nil {
			t.Errorf("CreateDomConfig failed %v", err)
		}
		defer os.Truncate(conf.Name(), 0)

		result, err := ioutil.ReadFile(conf.Name())
		if err != nil {
			t.Errorf("reading conf file failed %v", err)
		}

		if string(result) != `# This file is automatically generated by domainmgr
[msg]
  timestamp = "on"

[machine]
  type = "pc-q35-3.1"
  usb = "off"
  dump-guest-core = "off"
  accel = "kvm"
  vmport = "off"
  kernel-irqchip = "on"
  firmware = "/usr/lib/xen/boot/ovmf.bin"
  kernel = "/boot/kernel"
  initrd = "/boot/ramdisk"
  append = "init=/bin/sh"


[global]
  driver = "kvm-pit"
  property = "lost_tick_policy"
  value = "delay"

[global]
  driver = "ICH9-LPC"
  property = "disable_s3"
  value = "1"

[global]
  driver = "ICH9-LPC"
  property = "disable_s4"
  value = "1"

[rtc]
  base = "localtime"
  driftfix = "slew"

[device]
  driver = "intel-iommu"

[realtime]
  mlock = "off"

[chardev "charmonitor"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/qmp"
  server = "on"
  wait = "off"

[mon "monitor"]
  chardev = "charmonitor"
  mode = "control"

[chardev "charlistener"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/listener.qmp"
  server = "on"
  wait = "off"

[mon "listener"]
  chardev = "charlistener"
  mode = "control"

[memory]
  size = "10240"

[smp-opts]
  cpus = "2"
  sockets = "1"
  cores = "2"
  threads = "1"

[device]
  driver = "virtio-serial"
  addr = "3"

[chardev "charserial0"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/cons"
  server = "on"
  wait = "off"
  logfile = "/dev/fd/1"
  logappend = "on"

[device]
  driver = "virtconsole"
  chardev = "charserial0"
  name = "org.lfedge.eve.console.0"


#[device "video0"]
#  driver = "qxl-vga"
#  ram_size = "67108864"
#  vram_size = "67108864"
#  vram64_size_mb = "0"
#  vgamem_mb = "16"
#  max_outputs = "1"
#  bus = "pcie.0"
#  addr = "0x1"
[device "video0"]
  driver = "cirrus-vga"
  vgamem_mb = "16"
  bus = "pcie.0"
  addr = "0x1"

[device "pci.2"]
  driver = "pcie-root-port"
  port = "12"
  chassis = "2"
  bus = "pcie.0"
  addr = "0x2"

[device "usb"]
  driver = "qemu-xhci"
  p2 = "15"
  p3 = "15"
  bus = "pci.2"
  addr = "0x0"

[device "input0"]
  driver = "usb-tablet"
  bus = "usb.0"
  port = "1"


[device "pci.4"]
  driver = "pcie-root-port"
  port = "14"
  chassis = "4"
  bus = "pcie.0"
  addr = "4"

[drive "drive-virtio-disk0"]
  file = "/foo/bar.qcow2"
  format = "qcow2"
  if = "none"

[device "virtio-disk0"]
  driver = "virtio-blk-pci"
  scsi = "off"
  bus = "pci.4"
  addr = "0x0"
  drive = "drive-virtio-disk0"
  bootindex = "0"


[fsdev "fsdev1"]
  fsdriver = "local"
  security_model = "none"
  path = "/foo/container"

[device "fs1"]
  driver = "virtio-9p-pci"
  fsdev = "fsdev1"
  mount_tag = "hostshare"
  addr = "5"


[device "pci.6"]
  driver = "pcie-root-port"
  port = "16"
  chassis = "6"
  bus = "pcie.0"
  addr = "6"

[drive "drive-virtio-disk2"]
  file = "/foo/bar.raw"
  format = "raw"
  if = "none"

[device "virtio-disk2"]
  driver = "virtio-blk-pci"
  scsi = "off"
  bus = "pci.6"
  addr = "0x0"
  drive = "drive-virtio-disk2"
  bootindex = "2"


[drive "drive-sata0-3"]
  file = "/foo/cd.iso"
  format = "raw"
  if = "none"
  media = "cdrom"
  readonly = "on"

[device "sata0-0"]
  drive = "drive-sata0-3"
  driver = "ide-cd"
  bus = "ide.0"

[device "pci.7"]
  driver = "pcie-root-port"
  port = "17"
  chassis = "7"
  bus = "pcie.0"
  multifunction = "on"
  addr = "7"

[netdev "hostnet0"]
  type = "tap"
  ifname = "nbu1x1"
  br = "bn0"
  script = "/etc/xen/scripts/qemu-ifup"
  downscript = "no"

[device "net0"]
  driver = "virtio-net-pci"
  netdev = "hostnet0"
  mac = "6a:00:03:61:a6:90"
  bus = "pci.7"
  addr = "0x0"

[device "pci.8"]
  driver = "pcie-root-port"
  port = "18"
  chassis = "8"
  bus = "pcie.0"
  multifunction = "on"
  addr = "8"

[netdev "hostnet1"]
  type = "tap"
  ifname = "nbu1x2"
  br = "bn0"
  script = "/etc/xen/scripts/qemu-ifup"
  downscript = "no"

[device "net1"]
  driver = "virtio-net-pci"
  netdev = "hostnet1"
  mac = "6a:00:03:61:a6:91"
  bus = "pci.8"
  addr = "0x0"

[chardev "charserial-usr0"]
  backend = "tty"
  path = "/dev/ttyS0"

[device "serial-usr0"]
  driver = "isa-serial"
  chardev = "charserial-usr0"
` {
			t.Errorf("got an unexpected resulting config %s", string(result))
		}
	})

	config.VirtualizationMode = types.FML
	t.Run("amd64", func(t *testing.T) {
		conf.Seek(0, 0)
		if err := kvmIntel.CreateDomConfig("test", config, disks, &aa, conf); err != nil {
			t.Errorf("CreateDomConfig failed %v", err)
		}
		defer os.Truncate(conf.Name(), 0)

		result, err := ioutil.ReadFile(conf.Name())
		if err != nil {
			t.Errorf("reading conf file failed %v", err)
		}

		if string(result) != `# This file is automatically generated by domainmgr
[msg]
  timestamp = "on"

[machine]
  type = "pc-q35-3.1"
  usb = "off"
  dump-guest-core = "off"
  accel = "kvm"
  vmport = "off"
  kernel-irqchip = "on"
  firmware = "/usr/lib/xen/boot/ovmf.bin"
  kernel = "/boot/kernel"
  initrd = "/boot/ramdisk"
  append = "init=/bin/sh"


[global]
  driver = "kvm-pit"
  property = "lost_tick_policy"
  value = "delay"

[global]
  driver = "ICH9-LPC"
  property = "disable_s3"
  value = "1"

[global]
  driver = "ICH9-LPC"
  property = "disable_s4"
  value = "1"

[rtc]
  base = "localtime"
  driftfix = "slew"

[device]
  driver = "intel-iommu"

[realtime]
  mlock = "off"

[chardev "charmonitor"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/qmp"
  server = "on"
  wait = "off"

[mon "monitor"]
  chardev = "charmonitor"
  mode = "control"

[chardev "charlistener"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/listener.qmp"
  server = "on"
  wait = "off"

[mon "listener"]
  chardev = "charlistener"
  mode = "control"

[memory]
  size = "10240"

[smp-opts]
  cpus = "2"
  sockets = "1"
  cores = "2"
  threads = "1"

[device]
  driver = "virtio-serial"
  addr = "3"

[chardev "charserial0"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/cons"
  server = "on"
  wait = "off"
  logfile = "/dev/fd/1"
  logappend = "on"

[device]
  driver = "virtconsole"
  chardev = "charserial0"
  name = "org.lfedge.eve.console.0"


#[device "video0"]
#  driver = "qxl-vga"
#  ram_size = "67108864"
#  vram_size = "67108864"
#  vram64_size_mb = "0"
#  vgamem_mb = "16"
#  max_outputs = "1"
#  bus = "pcie.0"
#  addr = "0x1"
[device "video0"]
  driver = "cirrus-vga"
  vgamem_mb = "16"
  bus = "pcie.0"
  addr = "0x1"

[device "pci.2"]
  driver = "pcie-root-port"
  port = "12"
  chassis = "2"
  bus = "pcie.0"
  addr = "0x2"

[device "usb"]
  driver = "qemu-xhci"
  p2 = "15"
  p3 = "15"
  bus = "pci.2"
  addr = "0x0"

[device "input0"]
  driver = "usb-tablet"
  bus = "usb.0"
  port = "1"


[device "pci.4"]
  driver = "pcie-root-port"
  port = "14"
  chassis = "4"
  bus = "pcie.0"
  addr = "4"

[drive "drive-virtio-disk0"]
  file = "/foo/bar.qcow2"
  format = "qcow2"
  if = "none"

[device "virtio-disk0"]
  driver = "virtio-blk-pci"
  scsi = "off"
  bus = "pci.4"
  addr = "0x0"
  drive = "drive-virtio-disk0"
  bootindex = "0"


[fsdev "fsdev1"]
  fsdriver = "local"
  security_model = "none"
  path = "/foo/container"

[device "fs1"]
  driver = "virtio-9p-pci"
  fsdev = "fsdev1"
  mount_tag = "hostshare"
  addr = "5"


[device "pci.6"]
  driver = "pcie-root-port"
  port = "16"
  chassis = "6"
  bus = "pcie.0"
  addr = "6"

[drive "drive-virtio-disk2"]
  file = "/foo/bar.raw"
  format = "raw"
  if = "none"

[device "virtio-disk2"]
  driver = "virtio-blk-pci"
  scsi = "off"
  bus = "pci.6"
  addr = "0x0"
  drive = "drive-virtio-disk2"
  bootindex = "2"


[drive "drive-sata0-3"]
  file = "/foo/cd.iso"
  format = "raw"
  if = "none"
  media = "cdrom"
  readonly = "on"

[device "sata0-0"]
  drive = "drive-sata0-3"
  driver = "ide-cd"
  bus = "ide.0"

[device "pci.7"]
  driver = "pcie-root-port"
  port = "17"
  chassis = "7"
  bus = "pcie.0"
  multifunction = "on"
  addr = "7"

[netdev "hostnet0"]
  type = "tap"
  ifname = "nbu1x1"
  br = "bn0"
  script = "/etc/xen/scripts/qemu-ifup"
  downscript = "no"

[device "net0"]
  driver = "virtio-net-pci"
  netdev = "hostnet0"
  mac = "6a:00:03:61:a6:90"
  bus = "pci.7"
  addr = "0x0"

[device "pci.8"]
  driver = "pcie-root-port"
  port = "18"
  chassis = "8"
  bus = "pcie.0"
  multifunction = "on"
  addr = "8"

[netdev "hostnet1"]
  type = "tap"
  ifname = "nbu1x2"
  br = "bn0"
  script = "/etc/xen/scripts/qemu-ifup"
  downscript = "no"

[device "net1"]
  driver = "virtio-net-pci"
  netdev = "hostnet1"
  mac = "6a:00:03:61:a6:91"
  bus = "pci.8"
  addr = "0x0"

[chardev "charserial-usr0"]
  backend = "tty"
  path = "/dev/ttyS0"

[device "serial-usr0"]
  driver = "isa-serial"
  chardev = "charserial-usr0"
` {
			t.Errorf("got an unexpected resulting config %s", string(result))
		}
	})

	config.VirtualizationMode = types.HVM
	t.Run("arm64", func(t *testing.T) {
		conf.Seek(0, 0)
		if err := kvmArm.CreateDomConfig("test", config, disks, &aa, conf); err != nil {
			t.Errorf("CreateDomConfig failed %v", err)
		}
		defer os.Truncate(conf.Name(), 0)

		result, err := ioutil.ReadFile(conf.Name())
		if err != nil {
			t.Errorf("reading conf file failed %v", err)
		}

		if string(result) != `# This file is automatically generated by domainmgr
[msg]
  timestamp = "on"

[machine]
  type = "virt"
  usb = "off"
  dump-guest-core = "off"
  accel = "kvm:tcg"
  gic_version = "host"
  kernel = "/boot/kernel"
  initrd = "/boot/ramdisk"
  append = "init=/bin/sh"


[realtime]
  mlock = "off"

[chardev "charmonitor"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/qmp"
  server = "on"
  wait = "off"

[mon "monitor"]
  chardev = "charmonitor"
  mode = "control"

[chardev "charlistener"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/listener.qmp"
  server = "on"
  wait = "off"

[mon "listener"]
  chardev = "charlistener"
  mode = "control"

[memory]
  size = "10240"

[smp-opts]
  cpus = "2"
  sockets = "1"
  cores = "2"
  threads = "1"

[device]
  driver = "virtio-serial"
  addr = "3"

[chardev "charserial0"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/cons"
  server = "on"
  wait = "off"
  logfile = "/dev/fd/1"
  logappend = "on"

[device]
  driver = "virtconsole"
  chardev = "charserial0"
  name = "org.lfedge.eve.console.0"


#[device "video0"]
#  driver = "qxl-vga"
#  ram_size = "67108864"
#  vram_size = "67108864"
#  vram64_size_mb = "0"
#  vgamem_mb = "16"
#  max_outputs = "1"
#  bus = "pcie.0"
#  addr = "0x1"
[device "video0"]
  driver = "cirrus-vga"
  vgamem_mb = "16"
  bus = "pcie.0"
  addr = "0x1"

[device "pci.2"]
  driver = "pcie-root-port"
  port = "12"
  chassis = "2"
  bus = "pcie.0"
  addr = "0x2"

[device "usb"]
  driver = "qemu-xhci"
  p2 = "15"
  p3 = "15"
  bus = "pci.2"
  addr = "0x0"

[device "input0"]
  driver = "usb-tablet"
  bus = "usb.0"
  port = "1"


[device "pci.4"]
  driver = "pcie-root-port"
  port = "14"
  chassis = "4"
  bus = "pcie.0"
  addr = "4"

[drive "drive-virtio-disk0"]
  file = "/foo/bar.qcow2"
  format = "qcow2"
  if = "none"

[device "virtio-disk0"]
  driver = "virtio-blk-pci"
  scsi = "off"
  bus = "pci.4"
  addr = "0x0"
  drive = "drive-virtio-disk0"
  bootindex = "0"


[fsdev "fsdev1"]
  fsdriver = "local"
  security_model = "none"
  path = "/foo/container"

[device "fs1"]
  driver = "virtio-9p-pci"
  fsdev = "fsdev1"
  mount_tag = "hostshare"
  addr = "5"


[device "pci.6"]
  driver = "pcie-root-port"
  port = "16"
  chassis = "6"
  bus = "pcie.0"
  addr = "6"

[drive "drive-virtio-disk2"]
  file = "/foo/bar.raw"
  format = "raw"
  if = "none"

[device "virtio-disk2"]
  driver = "virtio-blk-pci"
  scsi = "off"
  bus = "pci.6"
  addr = "0x0"
  drive = "drive-virtio-disk2"
  bootindex = "2"


[drive "drive-sata0-3"]
  file = "/foo/cd.iso"
  format = "raw"
  if = "none"
  media = "cdrom"
  readonly = "on"

[device "sata0-0"]
  drive = "drive-sata0-3"
  driver = "usb-storage"


[device "pci.7"]
  driver = "pcie-root-port"
  port = "17"
  chassis = "7"
  bus = "pcie.0"
  multifunction = "on"
  addr = "7"

[netdev "hostnet0"]
  type = "tap"
  ifname = "nbu1x1"
  br = "bn0"
  script = "/etc/xen/scripts/qemu-ifup"
  downscript = "no"

[device "net0"]
  driver = "virtio-net-pci"
  netdev = "hostnet0"
  mac = "6a:00:03:61:a6:90"
  bus = "pci.7"
  addr = "0x0"

[device "pci.8"]
  driver = "pcie-root-port"
  port = "18"
  chassis = "8"
  bus = "pcie.0"
  multifunction = "on"
  addr = "8"

[netdev "hostnet1"]
  type = "tap"
  ifname = "nbu1x2"
  br = "bn0"
  script = "/etc/xen/scripts/qemu-ifup"
  downscript = "no"

[device "net1"]
  driver = "virtio-net-pci"
  netdev = "hostnet1"
  mac = "6a:00:03:61:a6:91"
  bus = "pci.8"
  addr = "0x0"

[chardev "charserial-usr0"]
  backend = "tty"
  path = "/dev/ttyS0"

[device "serial-usr0"]
  driver = "isa-serial"
  chardev = "charserial-usr0"
` {
			t.Errorf("got an unexpected resulting config %s", string(result))
		}
	})
}
func TestCreateDomConfig(t *testing.T) {
	config := types.DomainConfig{
		UUIDandVersion: types.UUIDandVersion{UUID: uuid.NewV4(), Version: "1.0"},
		VmConfig: types.VmConfig{
			Kernel:     "/boot/kernel",
			Ramdisk:    "/boot/ramdisk",
			ExtraArgs:  "init=/bin/sh",
			Memory:     1024 * 1024 * 10,
			VCpus:      2,
			VncDisplay: 5,
			VncPasswd:  "rosebud",
		},
		VifList: []types.VifInfo{
			{Bridge: "bn0", Mac: "6a:00:03:61:a6:90", Vif: "nbu1x1"},
			{Bridge: "bn0", Mac: "6a:00:03:61:a6:91", Vif: "nbu1x2"},
		},
		IoAdapterList: []types.IoAdapter{
			{Type: types.IoNetEth, Name: "eth0"},
			{Type: types.IoCom, Name: "COM1"},
		},
	}
	disks := []types.DiskStatus{
		{Format: zconfig.Format_QCOW2, FileLocation: "/foo/bar.qcow2", Devtype: "HDD"},
		{Format: zconfig.Format_CONTAINER, FileLocation: "/foo/container", Devtype: "Unclassified"},
		{Format: zconfig.Format_RAW, FileLocation: "/foo/bar.raw", Devtype: "HDD_EMPTY"},
		{Format: zconfig.Format_RAW, FileLocation: "/foo/cd.iso", Devtype: "CDROM"},
	}
	aa := types.AssignableAdapters{
		Initialized: true,
		IoBundleList: []types.IoBundle{
			{
				Type:            types.IoNetEth,
				AssignmentGroup: "eth0-1",
				Phylabel:        "eth0",
				Ifname:          "eth0",
				PciLong:         "0000:03:00.0",
				UsedByUUID:      config.UUIDandVersion.UUID,
			},
			{
				Type:            types.IoCom,
				AssignmentGroup: "COM1",
				Phylabel:        "COM1",
				Ifname:          "COM1",
				Serial:          "/dev/ttyS0",
				UsedByUUID:      config.UUIDandVersion.UUID,
			},
		},
	}
	conf, err := ioutil.TempFile("/tmp", "config")
	if err != nil {
		t.Errorf("Can't create config file for a domain %v", err)
	} else {
		defer os.Remove(conf.Name())
	}

	t.Run("amd64", func(t *testing.T) {
		conf.Seek(0, 0)
		if err := kvmIntel.CreateDomConfig("test", config, disks, &aa, conf); err != nil {
			t.Errorf("CreateDomConfig failed %v", err)
		}
		defer os.Truncate(conf.Name(), 0)

		result, err := ioutil.ReadFile(conf.Name())
		if err != nil {
			t.Errorf("reading conf file failed %v", err)
		}

		if string(result) != `# This file is automatically generated by domainmgr
[msg]
  timestamp = "on"

[machine]
  type = "pc-q35-3.1"
  usb = "off"
  dump-guest-core = "off"
  accel = "kvm"
  vmport = "off"
  kernel-irqchip = "on"
  firmware = "/usr/lib/xen/boot/ovmf.bin"
  kernel = "/boot/kernel"
  initrd = "/boot/ramdisk"
  append = "init=/bin/sh"


[global]
  driver = "kvm-pit"
  property = "lost_tick_policy"
  value = "delay"

[global]
  driver = "ICH9-LPC"
  property = "disable_s3"
  value = "1"

[global]
  driver = "ICH9-LPC"
  property = "disable_s4"
  value = "1"

[rtc]
  base = "localtime"
  driftfix = "slew"

[device]
  driver = "intel-iommu"

[realtime]
  mlock = "off"

[chardev "charmonitor"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/qmp"
  server = "on"
  wait = "off"

[mon "monitor"]
  chardev = "charmonitor"
  mode = "control"

[chardev "charlistener"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/listener.qmp"
  server = "on"
  wait = "off"

[mon "listener"]
  chardev = "charlistener"
  mode = "control"

[memory]
  size = "10240"

[smp-opts]
  cpus = "2"
  sockets = "1"
  cores = "2"
  threads = "1"

[device]
  driver = "virtio-serial"
  addr = "3"

[chardev "charserial0"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/cons"
  server = "on"
  wait = "off"
  logfile = "/dev/fd/1"
  logappend = "on"

[device]
  driver = "virtconsole"
  chardev = "charserial0"
  name = "org.lfedge.eve.console.0"


#[device "video0"]
#  driver = "qxl-vga"
#  ram_size = "67108864"
#  vram_size = "67108864"
#  vram64_size_mb = "0"
#  vgamem_mb = "16"
#  max_outputs = "1"
#  bus = "pcie.0"
#  addr = "0x1"
[device "video0"]
  driver = "cirrus-vga"
  vgamem_mb = "16"
  bus = "pcie.0"
  addr = "0x1"

[device "pci.2"]
  driver = "pcie-root-port"
  port = "12"
  chassis = "2"
  bus = "pcie.0"
  addr = "0x2"

[device "usb"]
  driver = "qemu-xhci"
  p2 = "15"
  p3 = "15"
  bus = "pci.2"
  addr = "0x0"

[device "input0"]
  driver = "usb-tablet"
  bus = "usb.0"
  port = "1"


[device "pci.4"]
  driver = "pcie-root-port"
  port = "14"
  chassis = "4"
  bus = "pcie.0"
  addr = "4"

[drive "drive-virtio-disk0"]
  file = "/foo/bar.qcow2"
  format = "qcow2"
  if = "none"

[device "virtio-disk0"]
  driver = "virtio-blk-pci"
  scsi = "off"
  bus = "pci.4"
  addr = "0x0"
  drive = "drive-virtio-disk0"
  bootindex = "0"


[fsdev "fsdev1"]
  fsdriver = "local"
  security_model = "none"
  path = "/foo/container"

[device "fs1"]
  driver = "virtio-9p-pci"
  fsdev = "fsdev1"
  mount_tag = "hostshare"
  addr = "5"


[device "pci.6"]
  driver = "pcie-root-port"
  port = "16"
  chassis = "6"
  bus = "pcie.0"
  addr = "6"

[drive "drive-virtio-disk2"]
  file = "/foo/bar.raw"
  format = "raw"
  if = "none"

[device "virtio-disk2"]
  driver = "virtio-blk-pci"
  scsi = "off"
  bus = "pci.6"
  addr = "0x0"
  drive = "drive-virtio-disk2"
  bootindex = "2"


[drive "drive-sata0-3"]
  file = "/foo/cd.iso"
  format = "raw"
  if = "none"
  media = "cdrom"
  readonly = "on"

[device "sata0-0"]
  drive = "drive-sata0-3"
  driver = "ide-cd"
  bus = "ide.0"

[device "pci.7"]
  driver = "pcie-root-port"
  port = "17"
  chassis = "7"
  bus = "pcie.0"
  multifunction = "on"
  addr = "7"

[netdev "hostnet0"]
  type = "tap"
  ifname = "nbu1x1"
  br = "bn0"
  script = "/etc/xen/scripts/qemu-ifup"
  downscript = "no"

[device "net0"]
  driver = "virtio-net-pci"
  netdev = "hostnet0"
  mac = "6a:00:03:61:a6:90"
  bus = "pci.7"
  addr = "0x0"

[device "pci.8"]
  driver = "pcie-root-port"
  port = "18"
  chassis = "8"
  bus = "pcie.0"
  multifunction = "on"
  addr = "8"

[netdev "hostnet1"]
  type = "tap"
  ifname = "nbu1x2"
  br = "bn0"
  script = "/etc/xen/scripts/qemu-ifup"
  downscript = "no"

[device "net1"]
  driver = "virtio-net-pci"
  netdev = "hostnet1"
  mac = "6a:00:03:61:a6:91"
  bus = "pci.8"
  addr = "0x0"

[device]
  driver = "vfio-pci"
  host = "03:00.0"
[chardev "charserial-usr0"]
  backend = "tty"
  path = "/dev/ttyS0"

[device "serial-usr0"]
  driver = "isa-serial"
  chardev = "charserial-usr0"
` {
			t.Errorf("got an unexpected resulting config %s", string(result))
		}
	})

	config.VirtualizationMode = types.FML
	t.Run("amd64", func(t *testing.T) {
		conf.Seek(0, 0)
		if err := kvmIntel.CreateDomConfig("test", config, disks, &aa, conf); err != nil {
			t.Errorf("CreateDomConfig failed %v", err)
		}
		defer os.Truncate(conf.Name(), 0)

		result, err := ioutil.ReadFile(conf.Name())
		if err != nil {
			t.Errorf("reading conf file failed %v", err)
		}

		if string(result) != `# This file is automatically generated by domainmgr
[msg]
  timestamp = "on"

[machine]
  type = "pc-q35-3.1"
  usb = "off"
  dump-guest-core = "off"
  accel = "kvm"
  vmport = "off"
  kernel-irqchip = "on"
  firmware = "/usr/lib/xen/boot/ovmf.bin"
  kernel = "/boot/kernel"
  initrd = "/boot/ramdisk"
  append = "init=/bin/sh"


[global]
  driver = "kvm-pit"
  property = "lost_tick_policy"
  value = "delay"

[global]
  driver = "ICH9-LPC"
  property = "disable_s3"
  value = "1"

[global]
  driver = "ICH9-LPC"
  property = "disable_s4"
  value = "1"

[rtc]
  base = "localtime"
  driftfix = "slew"

[device]
  driver = "intel-iommu"

[realtime]
  mlock = "off"

[chardev "charmonitor"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/qmp"
  server = "on"
  wait = "off"

[mon "monitor"]
  chardev = "charmonitor"
  mode = "control"

[chardev "charlistener"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/listener.qmp"
  server = "on"
  wait = "off"

[mon "listener"]
  chardev = "charlistener"
  mode = "control"

[memory]
  size = "10240"

[smp-opts]
  cpus = "2"
  sockets = "1"
  cores = "2"
  threads = "1"

[device]
  driver = "virtio-serial"
  addr = "3"

[chardev "charserial0"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/cons"
  server = "on"
  wait = "off"
  logfile = "/dev/fd/1"
  logappend = "on"

[device]
  driver = "virtconsole"
  chardev = "charserial0"
  name = "org.lfedge.eve.console.0"


#[device "video0"]
#  driver = "qxl-vga"
#  ram_size = "67108864"
#  vram_size = "67108864"
#  vram64_size_mb = "0"
#  vgamem_mb = "16"
#  max_outputs = "1"
#  bus = "pcie.0"
#  addr = "0x1"
[device "video0"]
  driver = "cirrus-vga"
  vgamem_mb = "16"
  bus = "pcie.0"
  addr = "0x1"

[device "pci.2"]
  driver = "pcie-root-port"
  port = "12"
  chassis = "2"
  bus = "pcie.0"
  addr = "0x2"

[device "usb"]
  driver = "qemu-xhci"
  p2 = "15"
  p3 = "15"
  bus = "pci.2"
  addr = "0x0"

[device "input0"]
  driver = "usb-tablet"
  bus = "usb.0"
  port = "1"


[device "pci.4"]
  driver = "pcie-root-port"
  port = "14"
  chassis = "4"
  bus = "pcie.0"
  addr = "4"

[drive "drive-virtio-disk0"]
  file = "/foo/bar.qcow2"
  format = "qcow2"
  if = "none"

[device "virtio-disk0"]
  driver = "virtio-blk-pci"
  scsi = "off"
  bus = "pci.4"
  addr = "0x0"
  drive = "drive-virtio-disk0"
  bootindex = "0"


[fsdev "fsdev1"]
  fsdriver = "local"
  security_model = "none"
  path = "/foo/container"

[device "fs1"]
  driver = "virtio-9p-pci"
  fsdev = "fsdev1"
  mount_tag = "hostshare"
  addr = "5"


[device "pci.6"]
  driver = "pcie-root-port"
  port = "16"
  chassis = "6"
  bus = "pcie.0"
  addr = "6"

[drive "drive-virtio-disk2"]
  file = "/foo/bar.raw"
  format = "raw"
  if = "none"

[device "virtio-disk2"]
  driver = "virtio-blk-pci"
  scsi = "off"
  bus = "pci.6"
  addr = "0x0"
  drive = "drive-virtio-disk2"
  bootindex = "2"


[drive "drive-sata0-3"]
  file = "/foo/cd.iso"
  format = "raw"
  if = "none"
  media = "cdrom"
  readonly = "on"

[device "sata0-0"]
  drive = "drive-sata0-3"
  driver = "ide-cd"
  bus = "ide.0"

[device "pci.7"]
  driver = "pcie-root-port"
  port = "17"
  chassis = "7"
  bus = "pcie.0"
  multifunction = "on"
  addr = "7"

[netdev "hostnet0"]
  type = "tap"
  ifname = "nbu1x1"
  br = "bn0"
  script = "/etc/xen/scripts/qemu-ifup"
  downscript = "no"

[device "net0"]
  driver = "virtio-net-pci"
  netdev = "hostnet0"
  mac = "6a:00:03:61:a6:90"
  bus = "pci.7"
  addr = "0x0"

[device "pci.8"]
  driver = "pcie-root-port"
  port = "18"
  chassis = "8"
  bus = "pcie.0"
  multifunction = "on"
  addr = "8"

[netdev "hostnet1"]
  type = "tap"
  ifname = "nbu1x2"
  br = "bn0"
  script = "/etc/xen/scripts/qemu-ifup"
  downscript = "no"

[device "net1"]
  driver = "virtio-net-pci"
  netdev = "hostnet1"
  mac = "6a:00:03:61:a6:91"
  bus = "pci.8"
  addr = "0x0"

[device]
  driver = "vfio-pci"
  host = "03:00.0"
[chardev "charserial-usr0"]
  backend = "tty"
  path = "/dev/ttyS0"

[device "serial-usr0"]
  driver = "isa-serial"
  chardev = "charserial-usr0"
` {
			t.Errorf("got an unexpected resulting config %s", string(result))
		}
	})

	config.VirtualizationMode = types.HVM
	t.Run("arm64", func(t *testing.T) {
		conf.Seek(0, 0)
		if err := kvmArm.CreateDomConfig("test", config, disks, &aa, conf); err != nil {
			t.Errorf("CreateDomConfig failed %v", err)
		}
		defer os.Truncate(conf.Name(), 0)

		result, err := ioutil.ReadFile(conf.Name())
		if err != nil {
			t.Errorf("reading conf file failed %v", err)
		}

		if string(result) != `# This file is automatically generated by domainmgr
[msg]
  timestamp = "on"

[machine]
  type = "virt"
  usb = "off"
  dump-guest-core = "off"
  accel = "kvm:tcg"
  gic_version = "host"
  kernel = "/boot/kernel"
  initrd = "/boot/ramdisk"
  append = "init=/bin/sh"


[realtime]
  mlock = "off"

[chardev "charmonitor"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/qmp"
  server = "on"
  wait = "off"

[mon "monitor"]
  chardev = "charmonitor"
  mode = "control"

[chardev "charlistener"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/listener.qmp"
  server = "on"
  wait = "off"

[mon "listener"]
  chardev = "charlistener"
  mode = "control"

[memory]
  size = "10240"

[smp-opts]
  cpus = "2"
  sockets = "1"
  cores = "2"
  threads = "1"

[device]
  driver = "virtio-serial"
  addr = "3"

[chardev "charserial0"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/cons"
  server = "on"
  wait = "off"
  logfile = "/dev/fd/1"
  logappend = "on"

[device]
  driver = "virtconsole"
  chardev = "charserial0"
  name = "org.lfedge.eve.console.0"


#[device "video0"]
#  driver = "qxl-vga"
#  ram_size = "67108864"
#  vram_size = "67108864"
#  vram64_size_mb = "0"
#  vgamem_mb = "16"
#  max_outputs = "1"
#  bus = "pcie.0"
#  addr = "0x1"
[device "video0"]
  driver = "cirrus-vga"
  vgamem_mb = "16"
  bus = "pcie.0"
  addr = "0x1"

[device "pci.2"]
  driver = "pcie-root-port"
  port = "12"
  chassis = "2"
  bus = "pcie.0"
  addr = "0x2"

[device "usb"]
  driver = "qemu-xhci"
  p2 = "15"
  p3 = "15"
  bus = "pci.2"
  addr = "0x0"

[device "input0"]
  driver = "usb-tablet"
  bus = "usb.0"
  port = "1"


[device "pci.4"]
  driver = "pcie-root-port"
  port = "14"
  chassis = "4"
  bus = "pcie.0"
  addr = "4"

[drive "drive-virtio-disk0"]
  file = "/foo/bar.qcow2"
  format = "qcow2"
  if = "none"

[device "virtio-disk0"]
  driver = "virtio-blk-pci"
  scsi = "off"
  bus = "pci.4"
  addr = "0x0"
  drive = "drive-virtio-disk0"
  bootindex = "0"


[fsdev "fsdev1"]
  fsdriver = "local"
  security_model = "none"
  path = "/foo/container"

[device "fs1"]
  driver = "virtio-9p-pci"
  fsdev = "fsdev1"
  mount_tag = "hostshare"
  addr = "5"


[device "pci.6"]
  driver = "pcie-root-port"
  port = "16"
  chassis = "6"
  bus = "pcie.0"
  addr = "6"

[drive "drive-virtio-disk2"]
  file = "/foo/bar.raw"
  format = "raw"
  if = "none"

[device "virtio-disk2"]
  driver = "virtio-blk-pci"
  scsi = "off"
  bus = "pci.6"
  addr = "0x0"
  drive = "drive-virtio-disk2"
  bootindex = "2"


[drive "drive-sata0-3"]
  file = "/foo/cd.iso"
  format = "raw"
  if = "none"
  media = "cdrom"
  readonly = "on"

[device "sata0-0"]
  drive = "drive-sata0-3"
  driver = "usb-storage"


[device "pci.7"]
  driver = "pcie-root-port"
  port = "17"
  chassis = "7"
  bus = "pcie.0"
  multifunction = "on"
  addr = "7"

[netdev "hostnet0"]
  type = "tap"
  ifname = "nbu1x1"
  br = "bn0"
  script = "/etc/xen/scripts/qemu-ifup"
  downscript = "no"

[device "net0"]
  driver = "virtio-net-pci"
  netdev = "hostnet0"
  mac = "6a:00:03:61:a6:90"
  bus = "pci.7"
  addr = "0x0"

[device "pci.8"]
  driver = "pcie-root-port"
  port = "18"
  chassis = "8"
  bus = "pcie.0"
  multifunction = "on"
  addr = "8"

[netdev "hostnet1"]
  type = "tap"
  ifname = "nbu1x2"
  br = "bn0"
  script = "/etc/xen/scripts/qemu-ifup"
  downscript = "no"

[device "net1"]
  driver = "virtio-net-pci"
  netdev = "hostnet1"
  mac = "6a:00:03:61:a6:91"
  bus = "pci.8"
  addr = "0x0"

[device]
  driver = "vfio-pci"
  host = "03:00.0"
[chardev "charserial-usr0"]
  backend = "tty"
  path = "/dev/ttyS0"

[device "serial-usr0"]
  driver = "isa-serial"
  chardev = "charserial-usr0"
` {
			t.Errorf("got an unexpected resulting config %s", string(result))
		}
	})
}

func TestGetDomsCPUMem(t *testing.T) {
	ctx := kvmContext{domains: map[string]int{"test1": 1}}
	res, err := ctx.GetDomsCPUMem()

	if err != nil {
		if strings.HasSuffix(err.Error(), "stat /proc: no such file or directory") {
			// skipping this test since we're clearly not running on Linux
			// XXX: is there a golang way to skip the test?
			return
		}
		t.Errorf("can't get domain statistics %v", err)
	}

	if len(res) != 1 || res["test1"].UsedMemoryPercent < 0 || res["test1"].UsedMemoryPercent > 100 ||
		res["test1"].CPUTotal != 0 || res["test1"].AvailableMemory < res["test1"].UsedMemory {
		t.Errorf("result from get domain statistics doesn't make sense %+v", res["test1"])
	}
}

func TestCreateDom(t *testing.T) {
	if exec.Command("qemu-system-x86_64", "--version").Run() != nil {
		// skipping this test since we're clearly not in a presence of qemu
		return
	}

	config := types.DomainConfig{
		UUIDandVersion: types.UUIDandVersion{UUID: uuid.NewV4(), Version: "1.0"},
		VmConfig: types.VmConfig{
			Kernel:             "/boot/kernel",
			Ramdisk:            "/boot/ramdisk",
			ExtraArgs:          "init=/bin/sh",
			Memory:             1024 * 1024 * 10,
			VCpus:              2,
			VncDisplay:         5,
			VncPasswd:          "rosebud",
			VirtualizationMode: types.HVM,
		},
		VifList: []types.VifInfo{
			{Bridge: "bn0", Mac: "6a:00:03:61:a6:90", Vif: "nbu1x1"},
			{Bridge: "bn0", Mac: "6a:00:03:61:a6:91", Vif: "nbu1x2"},
		},
		IoAdapterList: []types.IoAdapter{
			{Type: types.IoNetEth, Name: "eth0"},
			{Type: types.IoCom, Name: "COM1"},
		},
	}
	os.RemoveAll(kvmStateDir)

	conf, err := ioutil.TempFile("/tmp/", "config")
	if err != nil {
		t.Errorf("Can't create config file for a domain %v", err)
	} else {
		defer os.Remove(conf.Name())
	}
	ioutil.WriteFile(conf.Name(), []byte(`# This file is automatically generated by domainmgr
[msg]
  timestamp = "on"

[machine]
  type = "pc-q35-3.1"
  usb = "off"
  vmport = "off"
  dump-guest-core = "off"

[global]
  driver = "ICH9-LPC"
  property = "disable_s3"
  value = "1"

[global]
  driver = "ICH9-LPC"
  property = "disable_s4"
  value = "1"

[realtime]
  mlock = "off"

[rtc]
  base = "localtime"
  driftfix = "slew"

[chardev "charmonitor"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/qmp"
  server = "on"
  wait = "off"

[chardev "charlistener"]
  backend = "socket"
  path = "/var/run/hypervisor/kvm/test/listener.qmp"
  server = "on"
  wait = "off"

[mon "listener"]
  chardev = "charlistener"
  mode = "control"

[mon "monitor"]
  chardev = "charmonitor"
  mode = "control"

[memory]
  size = "1024"

[smp-opts]
  cpus = "2"
  sockets = "1"
  cores = "2"
  threads = "1"`), 0777)

	if _, err := hyperKvm.Create("test", conf.Name(), &config); err != nil {
		t.Errorf("Create domain config failed %v", err)
	}

	state, err := os.Open(kvmStateDir + "/test")
	if err != nil {
		t.Errorf("can't open stat dir for test domain %v", err)
	}

	names, err := state.Readdirnames(0)
	if err != nil || len(names) != 3 {
		t.Errorf("can't read stat dir for test domain or got unexpected content %v", err)
	}

	for _, e := range names {
		if _, found := map[string]bool{"pid": true, "qmp": true, "cons": true}[e]; !found {
			t.Errorf("got an unexpected entry %s in the stat dir for domain test", e)
		}
	}

	if err := hyperKvm.Start("test", 0); err != nil {
		t.Errorf("Start domain failed %v", err)
	}

	if err := hyperKvm.Stop("test", 0, true); err != nil {
		t.Errorf("Stop domain failed %v", err)
	}

	if err := hyperKvm.Delete("test", 0); err != nil {
		t.Errorf("Delete domain failed %v", err)
	}

	state, err = os.Open(kvmStateDir)
	if err != nil {
		t.Errorf("can't open stat dir for test domain %v", err)
	}

	names, err = state.Readdirnames(0)
	if err != nil || len(names) != 0 {
		t.Errorf("can't read stat dir for test domain or state dir is not empty after all domains are gone %v", err)
	}
}
