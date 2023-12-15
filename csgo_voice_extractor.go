package main

// #cgo CFLAGS: -Wall -g
// #include <stdlib.h>
// #include "decoder.h"
import "C"
import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unsafe"

	dem "github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/msg"
	wav "github.com/youpy/go-wav"
	"google.golang.org/protobuf/proto"
)

type ExitCode int

const (
	InvalidArguments      ExitCode = 10
	LoadCsgoLibError      ExitCode = 11
	DemoNotFound          ExitCode = 12
	ParsingError          ExitCode = 13
	UnsupportedAudioCodec ExitCode = 14
	NoVoiceDataFound      ExitCode = 15
	DecodingError         ExitCode = 16
	WavFileCreationError  ExitCode = 17
)

type UnsupportedCodec struct {
	Name    string
	Quality int32
	Version int32
}

type Error struct {
	message  string
	err      error
	exitCode ExitCode
}

var csgoLibPath string
var outputPath string
var demoPaths []string
var shouldExitOnFirstError = false
var unsupportedCodec *UnsupportedCodec

func (err *Error) Error() string {
	if err.err != nil {
		return fmt.Sprintf("%s\n%s", err.message, err.err.Error())
	}

	return fmt.Sprintf("%s\n", err.message)
}

func handleError(err Error) {
	fmt.Fprint(os.Stderr, err.Error())
	if shouldExitOnFirstError {
		os.Exit(int(err.exitCode))
	}
}

func handleInvalidArgument(message string, err error) {
	shouldExitOnFirstError = true

	handleError(Error{
		message:  message,
		err:      err,
		exitCode: InvalidArguments,
	})
}

func computeCsgoLibraryPath() {
	var ldLibraryPath string
	if runtime.GOOS == "darwin" {
		ldLibraryPath = os.Getenv("DYLD_LIBRARY_PATH")
	} else {
		ldLibraryPath = os.Getenv("LD_LIBRARY_PATH")
	}

	// The env variable LD_LIBRARY_PATH is mandatory only on unix platforms, see decoder.c for details.
	if ldLibraryPath == "" && runtime.GOOS != "windows" {
		if runtime.GOOS == "darwin" {
			handleInvalidArgument("DYLD_LIBRARY_PATH is missing, usage example: DYLD_LIBRARY_PATH=. csgove myDemo.dem", nil)
		}

		handleInvalidArgument("LD_LIBRARY_PATH is missing, usage example: LD_LIBRARY_PATH=. csgove myDemo.dem", nil)
	}

	var err error
	csgoLibPath, err = filepath.Abs(ldLibraryPath)
	if err != nil {
		handleInvalidArgument("Invalid library path provided", err)
	}

	csgoLibPath = strings.TrimSuffix(csgoLibPath, string(os.PathSeparator))

	_, err = os.Stat(csgoLibPath)
	if os.IsNotExist(err) {
		handleInvalidArgument("CSGO library folder doesn't exists", err)
	}

	var requiredFiles [2]string
	switch runtime.GOOS {
	case "windows":
		requiredFiles = [2]string{"vaudio_celt.dll", "tier0.dll"}
	case "darwin":
		requiredFiles = [2]string{"vaudio_celt.dylib", "libtier0.dylib"}
	default:
		requiredFiles = [2]string{"vaudio_celt_client.so", "libtier0_client.so"}
	}

	for _, requiredFile := range requiredFiles {
		_, err = os.Stat(csgoLibPath + string(os.PathSeparator) + requiredFile)
		if os.IsNotExist(err) {
			handleInvalidArgument("The required CSGO file "+requiredFile+" doesn't exists", err)
		}
	}
}

func computeOutputPathFlag() {
	if outputPath == "" {
		currentDirectory, err := os.Getwd()
		if err != nil {
			handleInvalidArgument("Failed to get current directory", err)
		}
		outputPath = currentDirectory
		return
	}

	var err error
	outputPath, err = filepath.Abs(outputPath)
	if err != nil {
		handleInvalidArgument("Invalid output path provided", err)
	}

	_, err = os.Stat(outputPath)
	if os.IsNotExist(err) {
		handleInvalidArgument("Output folder doesn't exists", err)
	}
}

func computeDemoPathsArgs() {
	demoPaths = flag.Args()
	if len(demoPaths) == 0 {
		handleInvalidArgument("No demo path provided", nil)
	}

	for _, demoPath := range demoPaths {
		if !strings.HasSuffix(demoPath, ".dem") {
			handleInvalidArgument(fmt.Sprintf("Invalid demo path: %s", demoPath), nil)
		}
	}
}

func parseArgs() {
	flag.StringVar(&outputPath, "output", "", "Output folder where WAV files will be written. Can be relative or absolute, default to the current directory.")
	flag.BoolVar(&shouldExitOnFirstError, "exit-on-first-error", false, "Exit the program on at the first error encountered, default to false.")
	flag.Parse()

	computeDemoPathsArgs()
	computeOutputPathFlag()
	computeCsgoLibraryPath()
}

func getPlayersVoiceData(demoFilePath string) (map[string][]byte, error) {
	var voiceDataPerPlayer = map[string][]byte{}
	file, err := os.Open(demoFilePath)
	if err != nil {
		return voiceDataPerPlayer, err
	}
	defer file.Close()

	parserConfig := dem.DefaultParserConfig
	parserConfig.AdditionalNetMessageCreators = map[int]dem.NetMessageCreator{
		int(msg.SVC_Messages_svc_VoiceData): func() proto.Message {
			return new(msg.CSVCMsg_VoiceData)
		},
		int(msg.SVC_Messages_svc_VoiceInit): func() proto.Message {
			return new(msg.CSVCMsg_VoiceInit)
		},
	}

	parser := dem.NewParserWithConfig(file, parserConfig)
	defer parser.Close()

	parser.RegisterNetMessageHandler(func(m *msg.CSVCMsg_VoiceInit) {
		if m.GetCodec() != "vaudio_celt" || m.GetQuality() != 5 || m.GetVersion() != 3 {
			unsupportedCodec = &UnsupportedCodec{
				Name:    m.GetCodec(),
				Quality: m.GetQuality(),
				Version: m.GetVersion(),
			}
			parser.Cancel()
		}
	})

	parser.RegisterNetMessageHandler(func(m *msg.CSVCMsg_VoiceData) {
		playerName := ""
		steamId := m.GetXuid()
		for _, player := range parser.GameState().Participants().All() {
			if player.SteamID64 == steamId {
				var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9]+`)
				playerName = nonAlphanumericRegex.ReplaceAllString(player.Name, "")
				break
			}
		}

		if playerName == "" {
			fmt.Println("Unable to find player's name with SteamID", steamId)
			return
		}

		playerId := fmt.Sprintf("%s_%d", playerName, steamId)
		if voiceDataPerPlayer[playerId] == nil {
			voiceDataPerPlayer[playerId] = make([]byte, 0)
		}

		voiceDataPerPlayer[playerId] = append(voiceDataPerPlayer[playerId], m.GetVoiceData()...)
	})

	err = parser.ParseToEnd()

	return voiceDataPerPlayer, err
}

func convertPcmFileToWavFile(pcmFilePath string, wavFilePath string) {
	data, err := os.ReadFile(pcmFilePath)
	if err != nil {
		handleError(Error{
			message:  "Failed to read PCM file",
			err:      err,
			exitCode: WavFileCreationError,
		})
		return
	}

	wavFile, err := os.Create(wavFilePath)
	if err != nil {
		handleError(Error{
			message:  "Couldn't create WAV file",
			err:      err,
			exitCode: WavFileCreationError,
		})
		return
	}
	defer wavFile.Close()

	var numSamples uint32 = uint32(len(data) / 2)
	var numChannels uint16 = 1
	var sampleRate uint32 = 22050
	var bitsPerSample uint16 = 16

	writer := wav.NewWriter(wavFile, numSamples, numChannels, sampleRate, bitsPerSample)
	_, err = writer.Write(data)
	if err != nil {
		handleError(Error{
			message:  "Couldn't create WAV file",
			err:      err,
			exitCode: WavFileCreationError,
		})
	}
}

func main() {
	parseArgs()

	cCsgoLibPath := C.CString(csgoLibPath)
	initAudioLibResult := C.Init(cCsgoLibPath)
	if initAudioLibResult != 0 {
		handleError(Error{
			message:  "Failed to initialize CSGO audio decoder",
			exitCode: LoadCsgoLibError,
		})
	}

	for _, demoPath := range demoPaths {
		fmt.Printf("Processing demo %s\n", demoPath)

		playersVoiceData, err := getPlayersVoiceData(demoPath)
		if unsupportedCodec != nil {
			handleError(Error{
				message:  fmt.Sprintf("unsupported audio codec: %s %d %d", unsupportedCodec.Name, unsupportedCodec.Quality, unsupportedCodec.Version),
				exitCode: UnsupportedAudioCodec,
			})
			continue
		}

		if err != nil {
			if _, isOpenFileError := err.(*os.PathError); isOpenFileError {
				handleError(Error{
					message:  fmt.Sprintf("Demo not found: %s", demoPath),
					err:      err,
					exitCode: DemoNotFound,
				})
			} else {
				handleError(Error{
					message:  fmt.Sprintf("Failed to parse demo: %s\n", demoPath),
					err:      err,
					exitCode: ParsingError,
				})
			}

			continue
		}

		if len(playersVoiceData) == 0 {
			handleError(Error{
				message:  fmt.Sprintf("No voice data found in demo %s\n", demoPath),
				exitCode: NoVoiceDataFound,
			})
			continue
		}

		demoName := strings.TrimSuffix(filepath.Base(demoPath), filepath.Ext(demoPath))
		for playerId, voiceData := range playersVoiceData {
			playerFileId := fmt.Sprintf("%s_%s", demoName, playerId)
			pcmTmpFile, err := os.CreateTemp("", "pcm.bin")
			pcmFilePath := pcmTmpFile.Name()
			if err != nil {
				handleError(Error{
					message:  fmt.Sprintf("Failed to write tmp file: %s\n", pcmFilePath),
					exitCode: DecodingError,
				})
				continue
			}
			defer os.Remove(pcmFilePath)
			cPcmFilePath := C.CString(pcmFilePath)
			cSize := C.int(len(voiceData))
			cData := (*C.uchar)(unsafe.Pointer(&voiceData[0]))
			result := C.Decode(cSize, cData, cPcmFilePath)
			if result != 0 {
				handleError(Error{
					message:  fmt.Sprintf("Failed to decode voice data: %d\n", result),
					exitCode: DecodingError,
				})
				continue
			}

			wavFilePath := fmt.Sprintf("%s/%s.wav", outputPath, playerFileId)
			convertPcmFileToWavFile(pcmFilePath, wavFilePath)
		}

		fmt.Printf("End processing demo %s\n", demoPath)
	}
}
