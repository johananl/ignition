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

package blackbox

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/flatcar-linux/ignition/internal/distro"
	"github.com/flatcar-linux/ignition/tests/types"
)

func run(ctx context.Context, command string, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, command, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed: %q: %v\n%s", command, err, out)
	}
	return out, nil
}

// Runs the command even if the context has exired. Should be used for cleanup
// operations
func runWithoutContext(command string, args ...string) ([]byte, error) {
	out, err := exec.Command(command, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed: %q: %v\n%s", command, err, out)
	}
	return out, nil
}

func prepareRootPartitionForPasswd(ctx context.Context, root *types.Partition) error {
	if err := mountPartition(ctx, root); err != nil {
		return err
	}
	defer umountPartition(root)

	mountPath := root.MountPath
	dirs := []string{
		filepath.Join(mountPath, "home"),
		filepath.Join(mountPath, "usr", "bin"),
		filepath.Join(mountPath, "usr", "sbin"),
		filepath.Join(mountPath, "usr", "lib64"),
		filepath.Join(mountPath, "etc"),
	}
	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}

	symlinks := []string{"lib64", "bin", "sbin"}
	for _, symlink := range symlinks {
		err := os.Symlink(
			filepath.Join(mountPath, "usr", symlink),
			filepath.Join(mountPath, symlink))
		if err != nil {
			return err
		}
	}

	// TODO: use the architecture, not hardcode amd64
	_, err := run(ctx, "cp", "bin/amd64/id-stub", filepath.Join(mountPath, distro.IdCmd()))
	if err != nil {
		return err
	}
	// TODO: needed for user_group_lookup.c
	_, err = run(ctx, "cp", "/lib64/libnss_files.so.2", filepath.Join(mountPath, "usr", "lib64"))
	return err
}

func getRootPartition(partitions []*types.Partition) *types.Partition {
	for _, p := range partitions {
		if p.Label == "ROOT" {
			return p
		}
	}
	return nil
}

func mountPartition(ctx context.Context, p *types.Partition) error {
	if p.MountPath == "" || p.Device == "" {
		return fmt.Errorf("Invalid partition for mounting %+v", p)
	}
	_, err := run(ctx, "mount", p.Device, p.MountPath)
	return err
}

// runGetExit runs the command and returns the exit status. It only returns an error when execing
// the command encounters an error. exec'd programs that exit with non-zero status will not return
// errors.
func runGetExit(cmd string, args ...string) (int, string, error) {
	tmp, err := exec.Command(cmd, args...).CombinedOutput()
	logs := string(tmp)
	if err == nil {
		return 0, logs, nil
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return -1, logs, err
	}
	status, ok2 := exitErr.Sys().(syscall.WaitStatus)
	if !ok2 {
		return -1, logs, err
	}
	return status.ExitStatus(), logs, nil
}

func umountPartition(p *types.Partition) error {
	if p.MountPath == "" || p.Device == "" {
		return fmt.Errorf("Invalid partition for unmounting %+v", p)
	}

	// sometimes umount returns exit status 32 when it succeeds. Retry in this
	// specific case. See https://github.com/coreos/bootengine/commit/8bf46fe78ec59bcd5148ce9ab8ec5fb805600151
	// for more context.
	for i := 0; i < 3; i++ {
		status, logs, err := runGetExit("umount", p.MountPath)
		if status == 0 {
			return nil
		}
		if err != nil {
			return fmt.Errorf("exec'ing `umount %s` failed: %v", p.MountPath, err)
		}
		if status != 32 {
			return fmt.Errorf("`umount %s` failed with exit status %d: %s", p.MountPath, status, logs)
		}
		// wait a sec to see if things clear up
		time.Sleep(time.Second)

		if unmounted, _, err := runGetExit("mountpoint", "-q", p.MountPath); err != nil {
			return fmt.Errorf("exec'ing `mountpoint -q %s` failed: %v", p.MountPath, err)
		} else if unmounted == 1 {
			return nil
		}
	}
	return fmt.Errorf("umount failed after 3 tries (exit status 32) for %s", p.MountPath)
}

// returns true if no error, false if error
func runIgnition(t *testing.T, ctx context.Context, stage, root, cwd string, appendEnv []string) error {
	args := []string{"-clear-cache", "-oem", "file", "-stage", stage,
		"-root", root, "-log-to-stdout", "--config-cache", filepath.Join(cwd, "ignition.json")}
	cmd := exec.CommandContext(ctx, "ignition", args...)
	t.Log("ignition", args)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), appendEnv...)
	out, err := cmd.CombinedOutput()
	t.Logf("PID: %d", cmd.Process.Pid)
	t.Logf("Ignition output:\n%s", string(out))
	if strings.Contains(string(out), "panic") {
		return fmt.Errorf("ignition panicked")
	}
	if strings.Contains(string(out), "CRITICAL") {
		return fmt.Errorf("found critical ignition log")
	}
	return err
}

// pickPartition will return the partition device corresponding to a
// partition with a given label on the given loop device
func pickPartition(device string, partitions []*types.Partition, label string) string {
	for _, p := range partitions {
		if p.Label == label {
			return fmt.Sprintf("%sp%d", device, p.Number)
		}
	}
	return ""
}

// setupDisk creates a backing file then loop mounts it. It sets up the partitions and filesystems on that loop device.
// It returns any error it encounters, but cleans up after itself if it errors out.
func setupDisk(ctx context.Context, disk *types.Disk, diskIndex int, imageSize int64, tmpDirectory string) (err error) {
	// attempt to create the file, will leave already existing files alone.
	// os.Truncate requires the file to already exist
	var (
		out *os.File
		tmp []byte
	)
	if out, err = os.Create(disk.ImageFile); err != nil {
		return err
	}
	defer func() {
		// Delete the image file if this function exits with an error
		if err != nil {
			os.Remove(disk.ImageFile)
		}
	}()
	out.Close()

	// Truncate the file to the given size
	if err = os.Truncate(disk.ImageFile, imageSize); err != nil {
		return err
	}

	// Attach the file to a loopback device
	tmp, err = run(ctx, "losetup", "-Pf", "--show", disk.ImageFile)
	if err != nil {
		return err
	}
	disk.Device = strings.TrimSpace(string(tmp))
	loopdev := disk.Device
	defer func() {
		if err != nil {
			destroyDevice(loopdev)
		}
	}()

	// Avoid race with kernel by waiting for loopDevice creation to complete
	if _, err = run(ctx, "udevadm", "settle"); err != nil {
		return fmt.Errorf("Settling devices: %v", err)
	}

	if err = createPartitionTable(ctx, disk.Device, disk.Partitions); err != nil {
		return err
	}

	for _, partition := range disk.Partitions {
		if partition.TypeCode == "blank" || partition.FilesystemType == "" || partition.FilesystemType == "swap" {
			continue
		}

		partition.MountPath = filepath.Join(tmpDirectory, fmt.Sprintf("hd%dp%d", diskIndex, partition.Number))
		if err = os.Mkdir(partition.MountPath, 0777); err != nil {
			return err
		}
		mountPath := partition.MountPath
		defer func() {
			// Delete the mount path if this function exits with an error
			if err != nil {
				os.RemoveAll(mountPath)
			}
		}()

		partition.Device = fmt.Sprintf("%sp%d", disk.Device, partition.Number)
		if err = formatPartition(ctx, partition); err != nil {
			return err
		}
	}
	return nil
}

func destroyDevice(loopDevice string) error {
	_, err := runWithoutContext("losetup", "-d", loopDevice)
	return err
}

func formatPartition(ctx context.Context, partition *types.Partition) error {
	var mkfs string
	var opts, label, uuid []string

	switch partition.FilesystemType {
	case "vfat":
		mkfs = "mkfs.vfat"
		label = []string{"-n", partition.FilesystemLabel}
		uuid = []string{"-i", partition.FilesystemUUID}
	case "ext2", "ext4":
		mkfs = "mke2fs"
		opts = []string{
			"-t", partition.FilesystemType, "-b", "4096",
			"-i", "4096", "-I", "128", "-e", "remount-ro",
		}
		label = []string{"-L", partition.FilesystemLabel}
		uuid = []string{"-U", partition.FilesystemUUID}
	case "btrfs":
		mkfs = "mkfs.btrfs"
		label = []string{"--label", partition.FilesystemLabel}
		uuid = []string{"--uuid", partition.FilesystemUUID}
	case "xfs":
		mkfs = "mkfs.xfs"
		label = []string{"-L", partition.FilesystemLabel}
		uuid = []string{"-m", "uuid=" + partition.FilesystemUUID}
	case "swap":
		mkfs = "mkswap"
		label = []string{"-L", partition.FilesystemLabel}
		uuid = []string{"-U", partition.FilesystemUUID}
	default:
		if partition.FilesystemType == "blank" ||
			partition.FilesystemType == "" {
			return nil
		}
		return fmt.Errorf("Unknown partition: %v", partition.FilesystemType)
	}

	if partition.FilesystemLabel != "" {
		opts = append(opts, label...)
	}
	if partition.FilesystemUUID != "" {
		opts = append(opts, uuid...)
	}
	opts = append(opts, partition.Device)

	_, err := run(ctx, mkfs, opts...)
	if err != nil {
		return err
	}

	if (partition.FilesystemType == "ext2" || partition.FilesystemType == "ext4") && partition.TypeCode == "coreos-usr" {
		// this is done to mirror the functionality from disk_util
		opts := []string{
			"-U", "clear", "-T", "20091119110000", "-c", "0", "-i", "0",
			"-m", "0", "-r", "0", "-e", "remount-ro", partition.Device,
		}
		_, err = run(ctx, "tune2fs", opts...)
		if err != nil {
			return err
		}
	}
	return nil
}

func createPartitionTable(ctx context.Context, imageFile string, partitions []*types.Partition) error {
	opts := []string{imageFile}
	hybrids := []int{}
	for _, p := range partitions {
		if p.TypeCode == "blank" || p.Length == 0 {
			continue
		}
		opts = append(opts, fmt.Sprintf(
			"--new=%d:%d:+%d", p.Number, p.Offset, p.Length))
		opts = append(opts, fmt.Sprintf(
			"--change-name=%d:%s", p.Number, p.Label))
		if p.TypeGUID != "" {
			opts = append(opts, fmt.Sprintf(
				"--typecode=%d:%s", p.Number, p.TypeGUID))
		}
		if p.GUID != "" {
			opts = append(opts, fmt.Sprintf(
				"--partition-guid=%d:%s", p.Number, p.GUID))
		}
		if p.Hybrid {
			hybrids = append(hybrids, p.Number)
		}
	}
	if len(hybrids) > 0 {
		if len(hybrids) > 3 {
			return fmt.Errorf("Can't have more than three hybrids")
		} else {
			opts = append(opts, fmt.Sprintf("-h=%s", intJoin(hybrids, ":")))
		}
	}
	_, err := run(ctx, "sgdisk", opts...)
	return err
}

func updateTypeGUID(partition *types.Partition) error {
	partitionTypes := map[string]string{
		"coreos-resize":   "3884DD41-8582-4404-B9A8-E9B84F2DF50E",
		"data":            "0FC63DAF-8483-4772-8E79-3D69D8477DE4",
		"coreos-rootfs":   "5DFBF5F4-2848-4BAC-AA5E-0D9A20B745A6",
		"bios":            "21686148-6449-6E6F-744E-656564454649",
		"efi":             "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
		"coreos-reserved": "C95DC21A-DF0E-4340-8D7B-26CBFA9A03E0",
	}

	if partition.TypeCode == "" || partition.TypeCode == "blank" {
		return nil
	}

	partition.TypeGUID = partitionTypes[partition.TypeCode]
	if partition.TypeGUID == "" {
		return fmt.Errorf("Unknown TypeCode: %s", partition.TypeCode)
	}
	return nil
}

func intJoin(ints []int, delimiter string) string {
	strArr := []string{}
	for _, i := range ints {
		strArr = append(strArr, strconv.Itoa(i))
	}
	return strings.Join(strArr, delimiter)
}

func removeEmpty(strings []string) []string {
	var r []string
	for _, str := range strings {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}

func createFilesForPartitions(ctx context.Context, partitions []*types.Partition) error {
	for _, partition := range partitions {
		if partition.FilesystemType == "swap" || partition.FilesystemType == "" || partition.FilesystemType == "blank" {
			continue
		}
		if err := mountPartition(ctx, partition); err != nil {
			return err
		}
		defer umountPartition(partition)

		err := createDirectoriesFromSlice(partition.MountPath, partition.Directories)
		if err != nil {
			return err
		}
		createFilesFromSlice(partition.MountPath, partition.Files)
		if err != nil {
			return err
		}
		createLinksFromSlice(partition.MountPath, partition.Links)
		if err != nil {
			return err
		}
	}
	return nil
}

func createFilesFromSlice(basedir string, files []types.File) error {
	for _, file := range files {
		err := os.MkdirAll(filepath.Join(
			basedir, file.Directory), 0755)
		if err != nil {
			return err
		}
		f, err := os.Create(filepath.Join(
			basedir, file.Directory, file.Name))
		if err != nil {
			return err
		}
		defer f.Close()
		if file.Contents != "" {
			writer := bufio.NewWriter(f)
			_, err := writer.WriteString(file.Contents)
			if err != nil {
				return err
			}
			writer.Flush()
		}
	}
	return nil
}

func createDirectoriesFromSlice(basedir string, dirs []types.Directory) error {
	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(
			basedir, dir.Directory), 0755)
		if err != nil {
			return err
		}
		err = os.Mkdir(filepath.Join(
			basedir, dir.Directory, dir.Name), os.FileMode(dir.Mode))
		if err != nil {
			return err
		}
	}
	return nil
}

func createLinksFromSlice(basedir string, links []types.Link) error {
	for _, link := range links {
		err := os.MkdirAll(filepath.Join(
			basedir, link.Directory), 0755)
		if err != nil {
			return err
		}
		if link.Hard {
			err = os.Link(link.Target, filepath.Join(basedir, link.Directory, link.Name))
		} else {
			err = os.Symlink(link.Target, filepath.Join(basedir, link.Directory, link.Name))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func setExpectedPartitionsDrive(actual []*types.Partition, expected []*types.Partition) {
	for _, a := range actual {
		for _, e := range expected {
			if a.Number == e.Number {
				e.MountPath = a.MountPath
				e.Device = a.Device
				break
			}
		}
	}
}
