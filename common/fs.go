package common

import "os"

func CreateWavFile(wavFilePath string) (*os.File, error) {
	file, err := os.Create(wavFilePath)
	if err != nil {
		HandleError(Error{
			Message:  "Couldn't create WAV file",
			Err:      err,
			ExitCode: WavFileCreationError,
		})
	}

	return file, err
}
