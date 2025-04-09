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
var mode string

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
	flag.StringVar(&mode, "mode", string(common.ModeSplitCompact), "Output mode. Can be 'split-compact', 'split-full' or 'single-full'. Default to 'split-compact'.")
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

func processDemoFile(demoPath string) {
	fmt.Printf("Processing demo %s\n", demoPath)

	file, err := os.Open(demoPath)
	if err != nil {
		if _, isOpenFileError := err.(*os.PathError); isOpenFileError {
			common.HandleError(common.NewError(
				fmt.Sprintf("Demo not found: %s", demoPath),
				err,
				common.DemoNotFound))
		} else {
			common.HandleError(common.NewError(
				fmt.Sprintf("Failed to open demo: %s", demoPath),
				err,
				common.OpenDemoError))
		}
		return
	}
	defer file.Close()

	timestamp, err := getDemoTimestamp(file, demoPath)
	if err != nil {
		common.HandleError(common.NewError(
			fmt.Sprintf("Failed to read demo timestamp: %s", demoPath),
			err,
			common.OpenDemoError))
		return
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		common.HandleError(common.NewError(
			fmt.Sprintf("Failed to reset demo file pointer: %s", demoPath),
			err,
			common.OpenDemoError))
		return
	}

	options := common.ExtractOptions{
		DemoPath:   demoPath,
		DemoName:   strings.TrimSuffix(filepath.Base(demoPath), filepath.Ext(demoPath)),
		File:       file,
		OutputPath: outputPath,
		Mode:       common.Mode(mode),
	}

	switch timestamp {
	case "HL2DEMO":
		csgo.Extract(options)
	case "PBDEMS2":
		cs2.Extract(options)
	default:
		common.HandleError(common.NewError(
			fmt.Sprintf("Unsupported demo format: %s", timestamp),
			nil,
			common.UnsupportedDemoFormat))
	}

	fmt.Printf("End processing demo %s\n", demoPath)
}

func main() {
	parseArgs()

	for _, demoPath := range demoPaths {
		processDemoFile(demoPath)
	}
}
