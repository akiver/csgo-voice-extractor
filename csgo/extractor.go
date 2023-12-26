package csgo

// #cgo CFLAGS: -Wall -g
// #include <stdlib.h>
// #include "decoder.h"
import "C"
import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/akiver/csgo-voice-extractor/common"
	dem "github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/msg"
	wav "github.com/youpy/go-wav"
	"google.golang.org/protobuf/proto"
)

var demoPaths []string
var unsupportedCodec *common.UnsupportedCodec

func getPlayersVoiceData(file *os.File) (map[string][]byte, error) {
	var voiceDataPerPlayer = map[string][]byte{}
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
			unsupportedCodec = &common.UnsupportedCodec{
				Name:    m.GetCodec(),
				Quality: m.GetQuality(),
				Version: m.GetVersion(),
			}
			parser.Cancel()
		}
	})

	parser.RegisterNetMessageHandler(func(m *msg.CSVCMsg_VoiceData) {
		playerID := common.GetPlayerID(parser, m.GetXuid())
		if playerID == "" {
			return
		}

		if voiceDataPerPlayer[playerID] == nil {
			voiceDataPerPlayer[playerID] = make([]byte, 0)
		}

		voiceDataPerPlayer[playerID] = append(voiceDataPerPlayer[playerID], m.GetVoiceData()...)
	})

	err := parser.ParseToEnd()

	return voiceDataPerPlayer, err
}

func convertPcmFileToWavFile(pcmFilePath string, wavFilePath string) {
	data, err := os.ReadFile(pcmFilePath)
	if err != nil {
		common.HandleError(common.Error{
			Message:  "Failed to read PCM file",
			Err:      err,
			ExitCode: common.WavFileCreationError,
		})
		return
	}

	wavFile, err := os.Create(wavFilePath)
	if err != nil {
		common.HandleError(common.Error{
			Message:  "Couldn't create WAV file",
			Err:      err,
			ExitCode: common.WavFileCreationError,
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
		common.HandleError(common.Error{
			Message:  "Couldn't write WAV file",
			Err:      err,
			ExitCode: common.WavFileCreationError,
		})
	}
}

func Extract(options common.ExtractOptions) {
	common.AssertLibraryFilesExist()

	cLibrariesPath := C.CString(common.LibrariesPath)
	initAudioLibResult := C.Init(cLibrariesPath)
	if initAudioLibResult != 0 {
		common.HandleError(common.Error{
			Message:  "Failed to initialize CSGO audio decoder",
			ExitCode: common.LoadCsgoLibError,
		})
		return
	}

	playersVoiceData, err := getPlayersVoiceData(options.File)
	if unsupportedCodec != nil {
		common.HandleError(common.Error{
			Message:  fmt.Sprintf("unsupported audio codec: %s %d %d", unsupportedCodec.Name, unsupportedCodec.Quality, unsupportedCodec.Version),
			Err:      err,
			ExitCode: common.UnsupportedAudioCodec,
		})
		return
	}

	demoPath := options.DemoPath
	isCorruptedDemo := errors.Is(err, dem.ErrUnexpectedEndOfDemo)
	if err != nil && !isCorruptedDemo {
		common.HandleError(common.Error{
			Message:  fmt.Sprintf("Failed to parse demo: %s\n", demoPath),
			Err:      err,
			ExitCode: common.ParsingError,
		})
		return
	}

	if len(playersVoiceData) == 0 {
		common.HandleError(common.Error{
			Message:  fmt.Sprintf("No voice data found in demo %s\n", demoPath),
			ExitCode: common.NoVoiceDataFound,
		})
		return
	}

	demoName := strings.TrimSuffix(filepath.Base(demoPath), filepath.Ext(demoPath))
	for playerId, voiceData := range playersVoiceData {
		playerFileName := fmt.Sprintf("%s_%s", demoName, playerId)
		pcmTmpFile, err := os.CreateTemp("", "pcm.bin")
		pcmFilePath := pcmTmpFile.Name()
		if err != nil {
			common.HandleError(common.Error{
				Message:  fmt.Sprintf("Failed to write tmp file: %s\n", pcmFilePath),
				ExitCode: common.DecodingError,
			})
			continue
		}
		defer os.Remove(pcmFilePath)
		cPcmFilePath := C.CString(pcmFilePath)
		cSize := C.int(len(voiceData))
		cData := (*C.uchar)(unsafe.Pointer(&voiceData[0]))
		result := C.Decode(cSize, cData, cPcmFilePath)
		if result != 0 {
			common.HandleError(common.Error{
				Message:  fmt.Sprintf("Failed to decode voice data: %d\n", result),
				ExitCode: common.DecodingError,
			})
			continue
		}

		wavFilePath := fmt.Sprintf("%s/%s.wav", options.OutputPath, playerFileName)
		convertPcmFileToWavFile(pcmFilePath, wavFilePath)
	}
}
