// Copyright (c) 2014,2015,2016 Docker, Inc.
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
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"

	"github.com/containers/virtcontainers/pkg/oci"
	"github.com/docker/docker/pkg/term"
	"github.com/urfave/cli"
)

var runCommand = cli.Command{
	Name:  "run",
	Usage: "create and run a container",
	ArgsUsage: `<container-id>

   <container-id> is your name for the instance of the container that you
   are starting. The name you provide for the container instance must be unique
   on your host.`,
	Description: `The run command creates an instance of a container for a bundle. The bundle
   is a directory with a specification file named "config.json" and a root
   filesystem.`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "bundle, b",
			Value: "",
			Usage: `path to the root of the bundle directory, defaults to the current directory`,
		},
		cli.StringFlag{
			Name:  "console",
			Value: "",
			Usage: "path to a pseudo terminal",
		},
		cli.StringFlag{
			Name:  "pid-file",
			Value: "",
			Usage: "specify the file to write the process id to",
		},
		cli.BoolFlag{
			Name:  "detach, d",
			Usage: "detach from the container's process",
		},
	},
	Action: func(context *cli.Context) error {
		return run(context)
	},
}

func run(context *cli.Context) error {
	runtimeConfig, ok := context.App.Metadata["runtimeConfig"].(oci.RuntimeConfig)
	if !ok {
		return errors.New("invalid runtime config")
	}

	consolePath := context.String("console")

	var wg sync.WaitGroup
	var console *Console
	var err error
	var consoleState *term.State

	// if consolePath is /dev/ptmx or /dev/pts/ptmx
	// means that we have to allocate a new pts
	if consolePath == ptmxPath || consolePath == ptsPtmxPath {
		console, err = newConsole()
		if err != nil {
			return err
		}

		// now consolePath is the slave pts
		consolePath = console.Path()
	}

	if err = create(context.Args().First(),
		context.String("bundle"),
		consolePath,
		context.String("pid-file"),
		runtimeConfig); err != nil {
		return err
	}

	detach := context.Bool("detach")

	if !detach {
		wg.Add(1)
		go io.Copy(console, os.Stdin)
		go func() {
			defer wg.Done()
			io.Copy(os.Stdout, console)
		}()

		// Save console state because it will be restored once container ends
		consoleState, err = term.SetRawTerminal(os.Stdin.Fd())
		if err != nil {
			return nil
		}
	}

	pod, err := start(context.Args().First())
	if err != nil {
		return err
	}

	if !detach {
		containers := pod.GetAllContainers()
		if len(containers) == 0 {
			return fmt.Errorf("There are no containers running in the pod: %s", pod.ID())
		}

		p, err := os.FindProcess(containers[0].GetPid())
		if err != nil {
			return err
		}

		ps, err := p.Wait()
		if err != nil {
			return fmt.Errorf("Process state %s: %s", ps.String(), err)
		}

		// delete container's resources
		if err = delete(containers[0].ID(), true); err != nil {
			return err
		}

		// wait for routines
		wg.Wait()

		// close and restore console
		console.Close()
		term.RestoreTerminal(os.Stdin.Fd(), consoleState)

		//runtime should forward container exit code to the system
		return cli.NewExitError("", ps.Sys().(syscall.WaitStatus).ExitStatus())
	}

	return nil
}
