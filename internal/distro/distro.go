// Copyright 2017 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package distro

import (
	"fmt"
	"os"
)

// Distro-specific settings that can be overridden at link time with e.g.
// -X github.com/flatcar-linux/ignition/internal/distro.mdadmCmd=/opt/bin/mdadm
var (
	// Device node directories and paths
	diskByLabelDir    = "/dev/disk/by-label"
	diskByPartUUIDDir = "/dev/disk/by-partuuid"
	oemDevicePath     = "/dev/disk/by-label/OEM"

	// File paths
	kernelCmdlinePath = "/proc/cmdline"
	// initramfs directory containing distro-provided base config
	systemConfigDir = "/usr/lib/ignition"
	// initramfs directory to check before retrieving file from OEM partition
	oemLookasideDir = "/usr/share/oem"

	// Helper programs
	chrootCmd     = "/usr/bin/chroot"
	groupaddCmd   = "/usr/sbin/groupadd"
	idCmd         = "/usr/bin/id"
	mdadmCmd      = "/usr/sbin/mdadm"
	mountCmd      = "/usr/bin/mount"
	sgdiskCmd     = "/usr/sbin/sgdisk"
	udevadmCmd    = "/usr/bin/udevadm"
	usermodCmd    = "/usr/sbin/usermod"
	useraddCmd    = "/usr/sbin/useradd"
	restoreconCmd = "/usr/sbin/restorecon"

	// Filesystem tools
	btrfsMkfsCmd = "/usr/sbin/mkfs.btrfs"
	ext4MkfsCmd  = "/usr/sbin/mkfs.ext4"
	swapMkfsCmd  = "/usr/sbin/mkswap"
	vfatMkfsCmd  = "/usr/sbin/mkfs.vfat"
	xfsMkfsCmd   = "/usr/sbin/mkfs.xfs"

	// Flags
	selinuxRelabel  = "false"
	blackboxTesting = "false"
)

func DiskByLabelDir() string    { return diskByLabelDir }
func DiskByPartUUIDDir() string { return diskByPartUUIDDir }
func OEMDevicePath() string     { return fromEnv("OEM_DEVICE", oemDevicePath) }

func KernelCmdlinePath() string { return kernelCmdlinePath }
func SystemConfigDir() string   { return fromEnv("SYSTEM_CONFIG_DIR", systemConfigDir) }
func OEMLookasideDir() string   { return fromEnv("OEM_LOOKASIDE_DIR", oemLookasideDir) }

func ChrootCmd() string     { return chrootCmd }
func GroupaddCmd() string   { return groupaddCmd }
func IdCmd() string         { return idCmd }
func MdadmCmd() string      { return mdadmCmd }
func MountCmd() string      { return mountCmd }
func SgdiskCmd() string     { return sgdiskCmd }
func UdevadmCmd() string    { return udevadmCmd }
func UsermodCmd() string    { return usermodCmd }
func UseraddCmd() string    { return useraddCmd }
func RestoreconCmd() string { return restoreconCmd }

func BtrfsMkfsCmd() string { return btrfsMkfsCmd }
func Ext4MkfsCmd() string  { return ext4MkfsCmd }
func SwapMkfsCmd() string  { return swapMkfsCmd }
func VfatMkfsCmd() string  { return vfatMkfsCmd }
func XfsMkfsCmd() string   { return xfsMkfsCmd }

func SelinuxRelabel() bool  { return bakedStringToBool(selinuxRelabel) }
func BlackboxTesting() bool { return bakedStringToBool(blackboxTesting) }

func fromEnv(nameSuffix, defaultValue string) string {
	value := os.Getenv("IGNITION_" + nameSuffix)
	if value != "" {
		return value
	}
	return defaultValue
}

func bakedStringToBool(s string) bool {
	// the linker only supports string args, so do some basic bool sensing
	if s == "true" || s == "1" {
		return true
	} else if s == "false" || s == "0" {
		return false
	} else {
		// if we got a bad compile flag, just crash and burn rather than assume
		panic(fmt.Sprintf("value '%s' cannot be interpreted as a boolean", s))
	}
}
