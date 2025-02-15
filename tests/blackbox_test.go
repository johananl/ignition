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
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	config "github.com/flatcar-linux/ignition/config/v2_4_experimental"
	"github.com/flatcar-linux/ignition/tests/register"
	"github.com/flatcar-linux/ignition/tests/types"

	// Register the tests
	_ "github.com/flatcar-linux/ignition/tests/registry"

	// UUID generation tool
	"github.com/pborman/uuid"
)

var (
	// testTimeout controls how long a given test is allowed to run before being
	// cancelled.
	testTimeout = time.Second * 60
	// somewhat of an abuse of contexts but go's got our hands tied
	killContext = context.TODO()

	// flag for listing all subtests that would be run without running them
	listSubtests = false
)

func TestMain(m *testing.M) {
	interruptChan := make(chan os.Signal, 3)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGTERM)
	tmp, killCancel := context.WithCancel(context.Background())
	killContext = tmp
	go func() {
		for {
			sig := <-interruptChan
			switch sig {
			case os.Interrupt, syscall.SIGTERM:
				killCancel()
			}
		}
	}()

	flag.BoolVar(&listSubtests, "list", false, "list tests that would be run without running them")
	flag.Parse()

	if !listSubtests {
		httpServer := &HTTPServer{}
		httpServer.Start()
		tftpServer := &TFTPServer{}
		tftpServer.Start()
	}
	os.Exit(m.Run())
}

func TestIgnitionBlackBox(t *testing.T) {
	for _, test := range register.Tests[register.PositiveTest] {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			if killContext.Err() != nil || (testing.Short() && test.ConfigMinVersion != test.ConfigVersion) {
				t.SkipNow()
			}
			if listSubtests {
				fmt.Println(t.Name())
				return
			}
			t.Parallel()
			err := outer(t, test, false)
			if err != nil {
				t.Error(err)
			}
		})
	}
}

func TestIgnitionBlackBoxNegative(t *testing.T) {
	for _, test := range register.Tests[register.NegativeTest] {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			if killContext.Err() != nil || (testing.Short() && test.ConfigMinVersion != test.ConfigVersion) {
				t.SkipNow()
			}
			if listSubtests {
				fmt.Println(t.Name())
				return
			}
			t.Parallel()
			err := outer(t, test, true)
			if err != nil {
				t.Error(err)
			}
		})
	}
}

func outer(t *testing.T, test types.Test, negativeTests bool) error {
	t.Log(test.Name)

	err := test.ReplaceAllUUIDVars()
	if err != nil {
		return err
	}

	ctx, cancelFunc := context.WithDeadline(killContext, time.Now().Add(testTimeout))
	defer cancelFunc()

	tmpDirectory, err := ioutil.TempDir("/var/tmp", "ignition-blackbox-")
	if err != nil {
		return fmt.Errorf("failed to create a temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDirectory)
	// the tmpDirectory must be 0755 or the tests will fail as the tool will
	// not have permissions to perform some actions in the mounted folders
	err = os.Chmod(tmpDirectory, 0755)
	if err != nil {
		return fmt.Errorf("failed to change mode of temp dir: %v", err)
	}

	oemLookasideDir := filepath.Join(tmpDirectory, "oem-lookaside")
	systemConfigDir := filepath.Join(tmpDirectory, "system")
	var rootPartition *types.Partition

	// Setup
	err = createFilesFromSlice(oemLookasideDir, test.OEMLookasideFiles)
	// Defer before the error handling because the createFilesFromSlice function
	// can fail after partially-creating things
	defer os.RemoveAll(oemLookasideDir)
	if err != nil {
		return err
	}
	err = createFilesFromSlice(systemConfigDir, test.SystemDirFiles)
	defer os.RemoveAll(systemConfigDir)
	if err != nil {
		return err
	}
	for i, disk := range test.In {
		// Set image file path
		disk.ImageFile = filepath.Join(tmpDirectory, fmt.Sprintf("hd%d", i))
		test.Out[i].ImageFile = disk.ImageFile

		// There may be more partitions created by Ignition, so look at the
		// expected output instead of the input to determine image size
		imageSize := test.Out[i].CalculateImageSize()
		if inSize := disk.CalculateImageSize(); inSize > imageSize {
			imageSize = inSize
		}

		// Finish data setup
		for _, part := range disk.Partitions {
			if part.GUID == "" {
				part.GUID = uuid.New()
				if err != nil {
					return err
				}
			}
			err := updateTypeGUID(part)
			if err != nil {
				return err
			}
		}

		disk.SetOffsets()
		for _, part := range test.Out[i].Partitions {
			err := updateTypeGUID(part)
			if err != nil {
				return err
			}
		}
		test.Out[i].SetOffsets()

		if err = setupDisk(ctx, &disk, i, imageSize, tmpDirectory); err != nil {
			return err
		}

		// Creation
		// Move value into the local scope, because disk.ImageFile and device
		// will change by the time this runs
		imageFile := disk.ImageFile
		device := disk.Device
		defer func() {
			if err := os.Remove(imageFile); err != nil {
				t.Errorf("couldn't remove %s: %v", imageFile, err)
			}
		}()
		defer func() {
			if err := destroyDevice(device); err != nil {
				t.Errorf("couldn't destroy device: %v", err)
			}
		}()

		test.Out[i].Device = disk.Device

		err = createFilesForPartitions(ctx, disk.Partitions)
		if err != nil {
			return err
		}

		// Mount device name substitution
		for _, d := range test.MntDevices {
			device := pickPartition(disk.Device, disk.Partitions, d.Label)
			// The device may not be on this disk, if it's not found here let's
			// assume we'll find it on another one and keep going
			if device != "" {
				test.Config = strings.Replace(test.Config, d.Substitution, device, -1)
			}
		}

		// Replace any instance of $disk<num> with the actual loop device
		// that got assigned to it
		test.Config = strings.Replace(test.Config, fmt.Sprintf("$disk%d", i), disk.Device, -1)

		if rootPartition == nil {
			rootPartition = getRootPartition(disk.Partitions)
		}
	}
	if rootPartition == nil {
		return fmt.Errorf("ROOT filesystem not found! A partition labeled ROOT is requred")
	}

	if strings.Contains(test.Config, "passwd") || strings.Contains(test.Config, "\"user\"") {
		if err := prepareRootPartitionForPasswd(ctx, rootPartition); err != nil {
			return err
		}
	}

	// Validation and cleanup deferral
	for i, disk := range test.Out {
		// Update out structure with mount points & devices
		setExpectedPartitionsDrive(test.In[i].Partitions, disk.Partitions)
	}

	// Let's make sure that all of the devices we needed to substitute names in
	// for were found
	for _, d := range test.MntDevices {
		if strings.Contains(test.Config, d.Substitution) {
			return fmt.Errorf("Didn't find a drive with label: %s", d.Substitution)
		}
	}

	// If we're not expecting the config to be bad, make sure it passes
	// validation.
	if !test.ConfigShouldBeBad {
		_, rpt, err := config.Parse([]byte(test.Config))
		if rpt.IsFatal() {
			return fmt.Errorf("test has bad config: %s", rpt.String())
		}
		if err != nil {
			return fmt.Errorf("error parsing config: %v", err)
		}
	}

	// Ignition config
	if err := ioutil.WriteFile(filepath.Join(tmpDirectory, "config.ign"), []byte(test.Config), 0666); err != nil {
		return fmt.Errorf("error writing config: %v", err)
	}

	// Ignition
	appendEnv := []string{
		"IGNITION_OEM_DEVICE=" + test.In[0].Partitions.GetPartition("OEM").Device,
		"IGNITION_OEM_LOOKASIDE_DIR=" + oemLookasideDir,
		"IGNITION_SYSTEM_CONFIG_DIR=" + systemConfigDir,
	}
	disksErr := runIgnition(t, ctx, "disks", rootPartition.MountPath, tmpDirectory, appendEnv)
	if !negativeTests && disksErr != nil {
		return disksErr
	}

	var filesErr error
	if disksErr == nil {
		// Even if we're running negative tests, we shouldn't run the files stage if the disks stage
		// failed. This is how Ignition was designed to be used.
		if err := mountPartition(ctx, rootPartition); err != nil {
			return err
		}
		filesErr = runIgnition(t, ctx, "files", rootPartition.MountPath, tmpDirectory, appendEnv)
		if err := umountPartition(rootPartition); err != nil {
			return err
		}
	}

	if !negativeTests && filesErr != nil {
		return filesErr
	}
	if negativeTests && disksErr == nil && filesErr == nil {
		return fmt.Errorf("Expected failure and ignition succeeded")
	}

	for _, disk := range test.Out {
		if !negativeTests {
			err = validateDisk(t, disk)
			if err != nil {
				return err
			}
			err = validateFilesystems(t, disk.Partitions)
			if err != nil {
				return err
			}
			validateFilesDirectoriesAndLinks(t, ctx, disk.Partitions)
		}
	}
	return nil
}
