// Copyright (c) 2017 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"syscall"

	vc "github.com/containers/virtcontainers"
)

func containerExists(containerID string) (bool, error) {
	podStatusList, err := vc.ListPod()
	if err != nil {
		return false, err
	}

	for _, podStatus := range podStatusList {
		if podStatus.ID == containerID {
			return true, nil
		}
	}

	return false, nil
}

func validCreateParams(containerID, bundlePath string) error {
	// container ID MUST be provided.
	if containerID == "" {
		return fmt.Errorf("Missing container ID")
	}

	// container ID MUST be unique.
	exist, err := containerExists(containerID)
	if err != nil {
		return err
	}
	if exist == true {
		return fmt.Errorf("ID already in use, unique ID should be provided")
	}

	// bundle path MUST be provided.
	if bundlePath == "" {
		return fmt.Errorf("Missing bundle path")
	}

	// bundle path MUST be valid.
	fileInfo, err := os.Stat(bundlePath)
	if err != nil {
		return fmt.Errorf("Invalid bundle path '%s': %s", bundlePath, err)
	}
	if fileInfo.IsDir() == false {
		return fmt.Errorf("Invalid bundle path '%s', it should be a directory", bundlePath)
	}

	return nil
}

func validContainer(containerID string) error {
	// container ID MUST be provided.
	if containerID == "" {
		return fmt.Errorf("Missing container ID")
	}

	// container ID MUST exist.
	exist, err := containerExists(containerID)
	if err != nil {
		return err
	}
	if exist == false {
		return fmt.Errorf("Container ID does not exist")
	}

	return nil
}

func processRunning(pid int) (bool, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}

	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false, nil
	}

	return true, nil
}

func updateStoppedContainer(podStatus vc.PodStatus) error {
	if len(podStatus.ContainersStatus) != 1 {
		return fmt.Errorf("ContainerStatus list from PodStatus is wrong, expecting only one ContainerStatus")
	}

	// Calling StopContainer allows to make sure the container is properly
	// stopped and removed from the pod. That way, the container's state is
	// updated to the expected "stopped" state.
	if _, err := vc.StopContainer(podStatus.ID, podStatus.ContainersStatus[0].ID); err != nil {
		return err
	}

	return nil
}
