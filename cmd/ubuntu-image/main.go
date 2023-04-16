package main

import (
	"fmt"
	"io"
	"os"

	"github.com/canonical/ubuntu-image/internal/commands"
	"github.com/canonical/ubuntu-image/internal/helper"
	"github.com/canonical/ubuntu-image/internal/statemachine"
	"github.com/jessevdk/go-flags"
)

// Version holds the ubuntu-image version number
// this is usually set at build time
var Version string

// osExit, captureStd, stateMachineInterface and imageType are helper variables for unit testing
var (
	osExit = os.Exit
	captureStd = helper.CaptureStd
	stateMachineInterface statemachine.SmInterface
	imageType string
)

const (
	stateMachineLongDesc = `Options for controlling the internal state machine.
Other than -w, these options are mutually exclusive. When -u or -t is given,
the state machine can be resumed later with -r, but -w must be given in that
case since the state is saved in a ubuntu-image.gob file in the working directory.`
)

func main() {
	// Initialize parser
	parser := flags.NewParser(&commands.Options, flags.Default)

	// Parse command line arguments
	args, err := parser.Parse()

	// Check for parsing errors, print usage and exit with error code 2
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			osExit(0)
		} else if flagsErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		parser.WriteHelp(os.Stderr)
		osExit(2)
	}

	// If we have args left after parsing, assume it's the image type
	if len(args) > 0 {
		imageType = args[0]
	}

	// Print version if requested
	if commands.Options.Version {
		fmt.Printf("ubuntu-image version %s\n", Version)
		osExit(0)
	}

	// Run state machine commands
	if commands.Options.StateMachine != nil {
		stateMachineInterface = statemachine.New(imageType)
		commands.RunStateMachineCommands(stateMachineInterface, commands.Options.StateMachine)
		osExit(0)
	}

	// Print usage if no subcommand is given
	parser.WriteHelp(os.Stderr)
	osExit(2)
func executeStateMachine(commonOpts *commands.CommonOpts, stateMachineOpts *commands.StateMachineOpts, ubuntuImageCommand *commands.UbuntuImageCommand) {
	// Set up the state machine
	if imageType == "snap" {
		stateMachine := new(statemachine.SnapStateMachine)
		stateMachine.Opts = ubuntuImageCommand.Snap.SnapOptsPassed
		stateMachine.Args = ubuntuImageCommand.Snap.SnapArgsPassed
		stateMachine.SetCommonOpts(commonOpts, stateMachineOpts)
		stateMachineInterface = stateMachine
	} else if imageType == "classic" {
		stateMachine := new(statemachine.ClassicStateMachine)
		stateMachine.Opts = ubuntuImageCommand.Classic.ClassicOptsPassed
		stateMachine.Args = ubuntuImageCommand.Classic.ClassicArgsPassed
		stateMachine.SetCommonOpts(commonOpts, stateMachineOpts)
		stateMachineInterface = stateMachine
	}

	// set up, run, and tear down the state machine
	if err := stateMachineInterface.Setup(); err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		osExit(1)
		return
	}

	if err := stateMachineInterface.Run(); err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		osExit(1)
		return
	}

	if err := stateMachineInterface.Teardown(); err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		osExit(1)
		return
	}

}

func main() {
	// instantiate structs for
	commonOpts := new(commands.CommonOpts)
	stateMachineOpts := new(commands.StateMachineOpts)
	ubuntuImageCommand := new(commands.UbuntuImageCommand)

	// set up the go-flags parser for command line options
	parser := flags.NewParser(ubuntuImageCommand, flags.Default)
	parser.AddGroup("State Machine Options", stateMachineLongDesc, stateMachineOpts)
	parser.AddGroup("Common Options", "Options common to both commands", commonOpts)

	// go-flags can be overzealous about printing errors that aren't actually errors
	// so we capture stdout/stderr while parsing and later decide whether to print
	stdout, restoreStdout, err := captureStd(&os.Stdout)
	if err != nil {
		fmt.Printf("Failed to capture stdout: %s\n", err.Error())
		osExit(1)
		return
	}
	defer restoreStdout()

	stderr, restoreStderr, err := captureStd(&os.Stderr)
	if err != nil {
		fmt.Printf("Failed to capture stderr: %s\n", err.Error())
		osExit(1)
		return
	}
	defer restoreStderr()

	// Parse the options provided and handle specific errors
	if _, err := parser.Parse(); err != nil {
		if e, ok := err.(*flags.Error); ok {
			switch e.Type {
			case flags.ErrHelp:
				restoreStdout()
				restoreStderr()
				readStdout, err := io.ReadAll(stdout)
				if err != nil {
					fmt.Printf("Error reading from stdout: %s\n", err.Error())
					osExit(1)
					return
				}
				fmt.Println(string(readStdout))
				osExit(0)
				return
			case flags.ErrCommandRequired:
				// if --resume was given, this is not an error
				if !stateMachineOpts.Resume && !commonOpts.Version {
					restoreStdout()
					restoreStderr()
					readStderr, err := io.ReadAll(stderr)
					if err != nil {
						fmt.Printf("Error reading from stderr: %s\n", err.Error())
						osExit(1)
						return
					}
					fmt.Printf("Error: %s\n", string(readStderr))
					osExit(1)
					return
				}
				break
			default:
				restoreStdout()
				restoreStderr()
				fmt.Printf("Error: %s\n", err.Error())
				osExit(1)
				return
			}
		}
	}

	// restore stdout
	restoreStdout()
	restoreStderr()

	// in case user only requested version number, print and exit
	if commonOpts.Version {
		// we expect Version to be supplied at build time or fetched from the snap environment
		if Version == "" {
			Version = os.Getenv("SNAP_VERSION")
		}
		fmt.Printf("ubuntu-image %s\n", Version)
		osExit(0)
		return
	}

	if parser.Command.Active != nil && imageType == "" {
		imageType = parser.Command.Active.Name
	}

	// let the state machine handle the image build
	executeStateMachine(commonOpts, stateMachineOpts, ubuntuImageCommand)
}
