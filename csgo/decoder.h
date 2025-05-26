#ifndef _AUDIO_H
#define _AUDIO_H

#if _WIN32
    #include <stdint.h>
    #include "dlfcn_win.h"
    #define LIB_NAME "vaudio_celt.dll"
#elif __APPLE__
    #include <dlfcn.h>
    #define LIB_NAME "vaudio_celt.dylib"
#else
    #include <dlfcn.h>
    #define LIB_NAME "vaudio_celt_client.so"
#endif

#define FRAME_SIZE 512
#define SAMPLE_RATE 22050
#define PACKET_SIZE 64

typedef struct CELTMode CELTMode;
typedef struct CELTDecoder CELTDecoder;
typedef struct CELTEncoder CELTEncoder;
typedef CELTMode* CeltModeCreateFunc(int32_t, int, int *error);
typedef CELTDecoder* CeltDecoderCreateCustomFunc(CELTMode*, int, int *error);
typedef int CeltDecodeFunc(CELTDecoder *st, const unsigned char *data, int len, int16_t *pcm, int frame_size);

int Init(const char *binariesPath);
int Release();
int Decode(int dataSize, unsigned char *data, char *pcmOut, int maxPcmBytes);

#endif
