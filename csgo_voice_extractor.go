package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/akiver/csgo-voice-extractor/common"
	"github.com/akiver/csgo-voice-extractor/cs2"
	"github.com/akiver/csgo-voice-extractor/csgo"
)

var outputPath string
var demoPaths []string

func computeOutputPathFlag() {
	if outputPath == "" {
		currentDirectory, err := os.Getwd()
		if err != nil {
			common.HandleInvalidArgument("Failed to get current directory", err)
		}
		outputPath = currentDirectory
		return
	}

	var err error
	outputPath, err = filepath.Abs(outputPath)
	if err != nil {
		common.HandleInvalidArgument("Invalid output path provided", err)
	}

	_, err = os.Stat(outputPath)
	if os.IsNotExist(err) {
		common.HandleInvalidArgument("Output folder doesn't exists", err)
	}
}

func computeDemoPathsArgs() {
	demoPaths = flag.Args()
	if len(demoPaths) == 0 {
		common.HandleInvalidArgument("No demo path provided", nil)
	}

	for _, demoPath := range demoPaths {
		if !strings.HasSuffix(demoPath, ".dem") {
			common.HandleInvalidArgument(fmt.Sprintf("Invalid demo path: %s", demoPath), nil)
		}
	}
}

func parseArgs() {
	flag.StringVar(&outputPath, "output", "", "Output folder where WAV files will be written. Can be relative or absolute, default to the current directory.")
	flag.BoolVar(&common.ShouldExitOnFirstError, "exit-on-first-error", false, "Exit the program on at the first error encountered, default to false.")
	flag.Parse()

	computeDemoPathsArgs()
	computeOutputPathFlag()
}

func getDemoTimestamp(file *os.File, demoPath string) (string, error) {
	buffer := make([]byte, 8)
	n, err := io.ReadFull(file, buffer)
	if err != nil {
		return "", err
	}

	timestamp := string(buffer[:n])
	timestamp = strings.TrimRight(timestamp, "\x00")

	return timestamp, nil
}

func main() {
	parseArgs()

	for _, demoPath := range demoPaths {
		fmt.Printf("Processing demo %s\n", demoPath)
		var networkProtocol int32

		file, err := os.Open(demoPath)
		if err != nil {
			if _, isOpenFileError := err.(*os.PathError); isOpenFileError {
				common.HandleError(common.Error{
					Message:  fmt.Sprintf("Demo not found: %s", demoPath),
					Err:      err,
					ExitCode: common.DemoNotFound,
				})
			} else {
				common.HandleError(common.Error{
					Message:  fmt.Sprintf("Failed to open demo: %s", demoPath),
					Err:      err,
					ExitCode: common.OpenDemoError,
				})
			}
			continue
		}
		defer file.Close()

		timestamp, err := getDemoTimestamp(file, demoPath)
		if err != nil {
			common.HandleError(common.Error{
				Message:  fmt.Sprintf("Failed to read demo timestamp: %s", demoPath),
				Err:      err,
				ExitCode: common.OpenDemoError,
			})
			continue
		}
		if timestamp == "PBDEMS2" {
			networkProtocol, err = cs2.GetDemoNetworkProtocol(demoPath, file)
			if err != nil {
				common.HandleError(common.Error{
					Message:  fmt.Sprintf("Failed to parse demo: %s\n", demoPath),
					Err:      err,
					ExitCode: common.ParsingError,
				})
				continue
			}
		}
		_, err = file.Seek(0, 0)
		if err != nil {
			common.HandleError(common.Error{
				Message:  fmt.Sprintf("Failed to reset demo file pointer: %s", demoPath),
				Err:      err,
				ExitCode: common.OpenDemoError,
			})
			continue
		}

		options := common.ExtractOptions{
			DemoPath:        demoPath,
			DemoName:        strings.TrimSuffix(filepath.Base(demoPath), filepath.Ext(demoPath)),
			File:            file,
			OutputPath:      outputPath,
			NetworkProtocol: networkProtocol,
		}

		switch timestamp {
		case "HL2DEMO":
			csgo.Extract(options)
		case "PBDEMS2":
			cs2.Extract(options)
		default:
			common.HandleError(common.Error{
				Message:  fmt.Sprintf("Unsupported demo format: %s", timestamp),
				ExitCode: common.UnsupportedDemoFormat,
			})
		}

		fmt.Printf("End processing demo %s\n", demoPath)
	}
}
