package csgo

// #cgo CFLAGS: -Wall -g
// #include <stdlib.h>
// #include "decoder.h"
import "C"
import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/akiver/csgo-voice-extractor/common"
	"github.com/go-audio/audio"
	goWav "github.com/go-audio/wav"
	dem "github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/msg"
	"github.com/youpy/go-wav"
	"google.golang.org/protobuf/proto"
)

const (
	SampleRate     = 22050
	BytesPerSample = 2   // 16-bit PCM
	PacketSize     = 64  // size of a single encoded voice packet in bytes
	FrameSize      = 512 // number of samples per frame after decoding
)

func createTempFile(prefix string) (string, error) {
	tmpFile, err := os.CreateTemp("", prefix)
	if err != nil {
		common.HandleError(common.Error{
			Message:  fmt.Sprintf("Failed to create temporary file: %s", err),
			ExitCode: common.DecodingError,
		})
		return "", err
	}

	filePath := tmpFile.Name()
	tmpFile.Close()

	return filePath, nil
}

func buildPlayerWavFilePath(playerID string, demoName string, outputPath string) string {
	return filepath.Join(outputPath, fmt.Sprintf("%s_%s.wav", demoName, playerID))
}

func getSegments(file *os.File) (map[string][]common.VoiceSegment, float64, error) {
	var segments = map[string][]common.VoiceSegment{}

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
			common.UnsupportedCodecError = &common.UnsupportedCodec{
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

		if segments[playerID] == nil {
			segments[playerID] = make([]common.VoiceSegment, 0)
		}
		segments[playerID] = append(segments[playerID], common.VoiceSegment{
			Data:      m.GetVoiceData(),
			Timestamp: parser.CurrentTime().Seconds(),
		})
	})

	err := parser.ParseToEnd()
	durationSeconds := parser.CurrentTime().Seconds()

	return segments, durationSeconds, err
}

func decodeVoiceData(data []byte) ([]byte, bool) {
	if len(data) == 0 {
		return nil, false
	}

	outputSamples := (len(data) / PacketSize) * FrameSize
	if outputSamples == 0 {
		outputSamples = FrameSize // fallback for very small packets
	}
	outputSize := outputSamples * 2 // 2 bytes per sample
	pcm := make([]byte, outputSize)

	cData := (*C.uchar)(unsafe.Pointer(&data[0]))
	cDataSize := C.int(len(data))
	cPcm := (*C.char)(unsafe.Pointer(&pcm[0]))
	cPcmSize := C.int(outputSize)

	written := C.Decode(cDataSize, cData, cPcm, cPcmSize)
	if written <= 0 {
		return nil, false
	}

	return pcm[:written], true
}

func generateAudioFileWithMergedVoices(segmentsPerPlayer map[string][]common.VoiceSegment, durationSeconds float64, demoName string, outputPath string) {
	wavFilePath := filepath.Join(outputPath, demoName+".wav")
	wavFile, err := common.CreateWavFile(wavFilePath)
	if err != nil {
		return
	}
	defer wavFile.Close()

	totalSamples := int(durationSeconds * float64(SampleRate))
	var numChannels uint16 = 1
	var bitsPerSample uint16 = 16
	writer := wav.NewWriter(wavFile, uint32(totalSamples), numChannels, SampleRate, bitsPerSample)

	type VoiceSegmentInfo struct {
		StartPosition int
		Data          []byte
		Length        int
	}

	voiceSegments := make([]VoiceSegmentInfo, 0)
	for _, segments := range segmentsPerPlayer {
		var previousEndPosition = 0
		for _, segment := range segments {
			startSample := int(segment.Timestamp * float64(SampleRate))
			startPosition := startSample * BytesPerSample

			if startPosition < previousEndPosition {
				startPosition = previousEndPosition
			}

			if startPosition >= totalSamples*BytesPerSample {
				fmt.Printf("Warning: Voice segment at %f seconds exceeds demo duration\n", segment.Timestamp)
				continue
			}

			samples, ok := decodeVoiceData(segment.Data)
			if !ok {
				common.HandleError(common.NewDecodingError("Failed to decode voice data", nil))
				continue
			}

			voiceSegments = append(voiceSegments, VoiceSegmentInfo{
				StartPosition: startPosition,
				Data:          samples,
				Length:        len(samples),
			})

			previousEndPosition = startPosition + len(samples)
		}
	}

	// process in small chunks to avoid high memory usage
	const chunkSize = 8192 * BytesPerSample
	for chunkStart := 0; chunkStart < totalSamples*BytesPerSample; chunkStart += chunkSize {
		chunkEnd := chunkStart + chunkSize
		if chunkEnd > totalSamples*BytesPerSample {
			chunkEnd = totalSamples * BytesPerSample
		}

		chunkLength := chunkEnd - chunkStart
		mixedChunk := make([]int32, chunkLength/BytesPerSample)
		activeSources := make([]int, chunkLength/BytesPerSample)

		// find segments that overlap with the current chunk
		for _, segment := range voiceSegments {
			segmentEnd := segment.StartPosition + segment.Length
			// check if this segment does not overlap with the current chunk
			if segmentEnd <= chunkStart || segment.StartPosition >= chunkEnd {
				continue
			}

			// calculate the start and end of the overlap
			overlapStart := segment.StartPosition
			if overlapStart < chunkStart {
				overlapStart = chunkStart
			}
			overlapEnd := segmentEnd
			if overlapEnd > chunkEnd {
				overlapEnd = chunkEnd
			}

			// add samples in the chunk and track active sources (players talking at the same time)
			for i := overlapStart; i < overlapEnd; i += BytesPerSample {
				sampleIndex := (i - chunkStart) / BytesPerSample
				sampleStartPosition := i - segment.StartPosition

				if sampleStartPosition >= 0 && sampleStartPosition < segment.Length-1 {
					sample := int32(int16(uint16(segment.Data[sampleStartPosition]) | uint16(segment.Data[sampleStartPosition+1])<<8))
					if sample != 0 { // ignore silence
						mixedChunk[sampleIndex] += sample
						activeSources[sampleIndex]++
					}
				}
			}
		}

		// normalize and mix samples in the chunk
		maxSampleValue := int32(1)
		chunkBytes := make([]byte, chunkLength)
		for i, sample := range mixedChunk {
			if sample == 0 || activeSources[i] == 0 {
				// write silence
				chunkBytes[i*BytesPerSample] = 0
				chunkBytes[i*BytesPerSample+1] = 0
				continue
			}

			// normalize the sample if several players are talking at the same time
			mixCoefficient := 1.0
			if activeSources[i] > 1 {
				mixCoefficient = 1.0 / math.Sqrt(float64(activeSources[i]))
			}

			mixedSample := float64(sample) * mixCoefficient
			if int32(math.Abs(mixedSample)) > maxSampleValue {
				maxSampleValue = int32(math.Abs(mixedSample))
			}

			// clip to int16 range because WAV format requires 16-bit samples
			if mixedSample > math.MaxInt16 {
				mixedSample = math.MaxInt16
			} else if mixedSample < -math.MaxInt16 {
				mixedSample = -math.MaxInt16
			}

			// convert back to bytes
			mixedInt16 := int16(mixedSample)
			chunkBytes[i*BytesPerSample] = byte(mixedInt16)
			chunkBytes[i*BytesPerSample+1] = byte(mixedInt16 >> 8)
		}

		_, err = writer.Write(chunkBytes)
		if err != nil {
			common.HandleError(common.Error{
				Message:  "Couldn't write WAV file",
				Err:      err,
				ExitCode: common.WavFileCreationError,
			})
		}
	}
}

func generateAudioFilesWithDemoLength(segmentsPerPlayer map[string][]common.VoiceSegment, durationSeconds float64, demoName string, outputPath string) {
	for playerID, segments := range segmentsPerPlayer {
		wavFilePath := buildPlayerWavFilePath(playerID, demoName, outputPath)
		totalSamples := int(durationSeconds * float64(SampleRate))

		wavFile, err := common.CreateWavFile(wavFilePath)
		if err != nil {
			return
		}
		defer wavFile.Close()

		var numChannels uint16 = 1
		var bitsPerSample uint16 = 16
		writer := wav.NewWriter(wavFile, uint32(totalSamples), numChannels, SampleRate, bitsPerSample)

		chunkSize := 8192 * BytesPerSample
		chunkBuffer := make([]byte, chunkSize)

		previousEndPosition := 0
		segmentIndex := 0
		for position := 0; position < totalSamples*BytesPerSample; position += chunkSize {
			// clear the chunk buffer
			for i := range chunkBuffer {
				chunkBuffer[i] = 0
			}

			currentChunkEnd := position + chunkSize
			if currentChunkEnd > totalSamples*BytesPerSample {
				currentChunkEnd = totalSamples * BytesPerSample
				chunkBuffer = chunkBuffer[:currentChunkEnd-position]
			}

			// find segments that overlap with the current chunk
			for segmentIndex < len(segments) {
				segment := segments[segmentIndex]
				startSample := int(segment.Timestamp * float64(SampleRate))
				startPosition := startSample * BytesPerSample

				if startPosition < previousEndPosition {
					startPosition = previousEndPosition
				}

				if startPosition >= currentChunkEnd {
					break
				}

				samples, ok := decodeVoiceData(segment.Data)
				if !ok {
					segmentIndex++
					continue
				}

				segmentEnd := startPosition + len(samples)

				// calculate the start and end of the overlap
				overlapStart := startPosition
				if overlapStart < position {
					overlapStart = position
				}
				overlapEnd := segmentEnd
				if overlapEnd > currentChunkEnd {
					overlapEnd = currentChunkEnd
				}

				// copy overlapping data to the chunk buffer
				if overlapStart < overlapEnd {
					srcOffset := overlapStart - startPosition
					destOffset := overlapStart - position
					copyLength := overlapEnd - overlapStart
					sampleCount := len(samples)
					chunkCount := len(chunkBuffer)

					if srcOffset >= 0 && srcOffset < sampleCount &&
						destOffset >= 0 && destOffset < chunkCount &&
						srcOffset+copyLength <= sampleCount &&
						destOffset+copyLength <= chunkCount {
						copy(chunkBuffer[destOffset:destOffset+copyLength], samples[srcOffset:srcOffset+copyLength])
					}
				}

				previousEndPosition = segmentEnd
				if segmentEnd > currentChunkEnd {
					break
				}
				segmentIndex++
			}

			_, err = writer.Write(chunkBuffer)
			if err != nil {
				common.HandleError(common.Error{
					Message:  "Couldn't write WAV file",
					Err:      err,
					ExitCode: common.WavFileCreationError,
				})
				return
			}
		}
	}
}

func generateAudioFilesWithCompactLength(segmentsPerPlayer map[string][]common.VoiceSegment, demoName string, options common.ExtractOptions) {
	for playerID, playerSegments := range segmentsPerPlayer {
		if len(playerSegments) == 0 {
			continue
		}

		wavFilePath := buildPlayerWavFilePath(playerID, demoName, options.OutputPath)
		outFile, err := common.CreateWavFile(wavFilePath)
		if err != nil {
			continue
		}
		defer outFile.Close()

		enc := goWav.NewEncoder(outFile, SampleRate, 16, 1, 1)
		defer enc.Close()

		for _, segment := range playerSegments {
			samples, ok := decodeVoiceData(segment.Data)
			if !ok {
				continue
			}

			if len(samples) > 0 {
				// convert to ints for WAV encoding
				numSamples := len(samples) / 2
				intSamples := make([]int, numSamples)
				for i := 0; i < numSamples; i++ {
					sample := int16(uint16(samples[i*2]) | uint16(samples[i*2+1])<<8)
					intSamples[i] = int(sample)
				}

				buf := &audio.IntBuffer{
					Data: intSamples,
					Format: &audio.Format{
						SampleRate:  SampleRate,
						NumChannels: 1,
					},
				}

				if err := enc.Write(buf); err != nil {
					common.HandleError(common.Error{
						Message:  "Couldn't write WAV file",
						Err:      err,
						ExitCode: common.WavFileCreationError,
					})
				}
			}
		}
	}
}

func Extract(options common.ExtractOptions) {
	common.AssertLibraryFilesExist()

	cLibrariesPath := C.CString(common.LibrariesPath)
	initAudioLibResult := C.Init(cLibrariesPath)
	C.free(unsafe.Pointer(cLibrariesPath))

	if initAudioLibResult != 0 {
		common.HandleError(common.Error{
			Message:  "Failed to initialize CSGO audio decoder",
			ExitCode: common.LoadCsgoLibError,
		})
		return
	}

	segmentsPerPlayer, durationSeconds, err := getSegments(options.File)
	common.AssertCodecIsSupported()

	demoPath := options.DemoPath
	isCorruptedDemo := errors.Is(err, dem.ErrUnexpectedEndOfDemo)
	isCanceled := errors.Is(err, dem.ErrCancelled)
	if err != nil && !isCorruptedDemo && !isCanceled {
		common.HandleError(common.Error{
			Message:  fmt.Sprintf("Failed to parse demo: %s\n", demoPath),
			Err:      err,
			ExitCode: common.ParsingError,
		})
		return
	}

	if isCanceled {
		return
	}

	if len(segmentsPerPlayer) == 0 {
		common.HandleError(common.Error{
			Message:  fmt.Sprintf("No voice data found in demo %s\n", demoPath),
			ExitCode: common.NoVoiceDataFound,
		})
		return
	}

	fmt.Println("Parsing done, generating audio files...")
	demoName := strings.TrimSuffix(filepath.Base(demoPath), filepath.Ext(demoPath))
	if options.Mode == common.ModeSingleFull {
		generateAudioFileWithMergedVoices(segmentsPerPlayer, durationSeconds, demoName, options.OutputPath)
	} else if options.Mode == common.ModeSplitFull {
		generateAudioFilesWithDemoLength(segmentsPerPlayer, durationSeconds, demoName, options.OutputPath)
	} else {
		generateAudioFilesWithCompactLength(segmentsPerPlayer, demoName, options)
	}
}
