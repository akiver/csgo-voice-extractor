package common

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var ShouldExitOnFirstError = false
var LibrariesPath string

type ExitCode int

type Error struct {
	Message  string
	Err      error
	ExitCode ExitCode
}

const (
	InvalidArguments      ExitCode = 10
	LoadCsgoLibError      ExitCode = 11
	DemoNotFound          ExitCode = 12
	ParsingError          ExitCode = 13
	UnsupportedAudioCodec ExitCode = 14
	NoVoiceDataFound      ExitCode = 15
	DecodingError         ExitCode = 16
	WavFileCreationError  ExitCode = 17
	OpenDemoError         ExitCode = 18
	UnsupportedDemoFormat ExitCode = 19
	MissingLibraryFiles   ExitCode = 20
)

type UnsupportedCodec struct {
	Name    string
	Quality int32
	Version int32
}

var UnsupportedCodecError *UnsupportedCodec

func (err *Error) Error() string {
	if err.Err != nil {
		return fmt.Sprintf("%s\n%s", err.Message, err.Err.Error())
	}

	return fmt.Sprintf("%s\n", err.Message)
}

func HandleError(err Error) Error {
	fmt.Fprint(os.Stderr, err.Error())
	if ShouldExitOnFirstError {
		os.Exit(int(err.ExitCode))
	}

	return err
}

func HandleInvalidArgument(message string, err error) Error {
	ShouldExitOnFirstError = true

	return HandleError(Error{
		Message:  message,
		Err:      err,
		ExitCode: InvalidArguments,
	})
}

func AssertLibraryFilesExist() {
	var ldLibraryPath string
	if runtime.GOOS == "darwin" {
		ldLibraryPath = os.Getenv("DYLD_LIBRARY_PATH")
	} else {
		ldLibraryPath = os.Getenv("LD_LIBRARY_PATH")
	}

	// The env variable LD_LIBRARY_PATH is mandatory only on unix platforms, see decoder.c for details.
	if ldLibraryPath == "" && runtime.GOOS != "windows" {
		if runtime.GOOS == "darwin" {
			HandleInvalidArgument("DYLD_LIBRARY_PATH is missing, usage example: DYLD_LIBRARY_PATH=. csgove myDemo.dem", nil)
		} else {
			HandleInvalidArgument("LD_LIBRARY_PATH is missing, usage example: LD_LIBRARY_PATH=. csgove myDemo.dem", nil)
		}
	}

	var err error
	LibrariesPath, err = filepath.Abs(ldLibraryPath)
	if err != nil {
		HandleInvalidArgument("Invalid library path provided", err)
	}

	LibrariesPath = strings.TrimSuffix(LibrariesPath, string(os.PathSeparator))

	_, err = os.Stat(LibrariesPath)
	if os.IsNotExist(err) {
		HandleInvalidArgument("Library folder doesn't exists", err)
	}

	var requiredFiles []string
	switch runtime.GOOS {
	case "windows":
		requiredFiles = []string{"vaudio_celt.dll", "tier0.dll", "opus.dll"}
	case "darwin":
		requiredFiles = []string{"vaudio_celt.dylib", "libtier0.dylib", "libvstdlib.dylib", "libopus.0.dylib"}
	default:
		requiredFiles = []string{"vaudio_celt_client.so", "libtier0_client.so", "libopus.so.0"}
	}

	for _, requiredFile := range requiredFiles {
		_, err = os.Stat(LibrariesPath + string(os.PathSeparator) + requiredFile)
		if os.IsNotExist(err) {
			ShouldExitOnFirstError = true
			HandleError(Error{
				Message:  "The required library file " + requiredFile + " doesn't exists",
				Err:      err,
				ExitCode: MissingLibraryFiles,
			})
		}
	}
}

func AssertCodeIsSupported() {
	if UnsupportedCodecError != nil {
		HandleError(Error{
			Message: fmt.Sprintf(
				"unsupported audio codec: %s %d %d",
				UnsupportedCodecError.Name,
				UnsupportedCodecError.Quality,
				UnsupportedCodecError.Version,
			),
			ExitCode: UnsupportedAudioCodec,
		})
	}
}
