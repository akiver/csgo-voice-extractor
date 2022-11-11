#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include "decoder.h"

void *handle;
CeltDecodeFunc* celtDecode;
CELTDecoder *decoder;

int Init(const char *csgoLibPath) {
    // The CSGO audio lib depends on an additional lib "tier0" which is not located on standard paths but in the CSGO folder.
    // It means that loading the audio lib without LD_LIBRARY_PATH would fail because it won't be able to find the tier0 lib.
    // That's why LD_LIBRARY_PATH must be set on unix, even the script used to start CSGO on unix does it (see csgo.sh in the game folder).
    // On Windows, LoadLibraryEx is able to load a DLL and its additional dependencies if they are in the same folder.
    #if _WIN32
        char csgoLibraryFullPath[1024];
        snprintf(csgoLibraryFullPath, sizeof(csgoLibraryFullPath), "%s\\%s", csgoLibPath, LIB_NAME);
        handle = dlopen(csgoLibraryFullPath, RTLD_LAZY);
    #else
        handle = dlopen(LIB_NAME, RTLD_LAZY);
    #endif
    
    if (!handle) {
        fprintf(stderr, "dlopen failed: %s\n", dlerror());
        return EXIT_FAILURE;
    }

    CeltModeCreateFunc* celtModeCreate = dlsym(handle, "celt_mode_create");
    if (celtModeCreate == NULL) {
        fprintf(stderr, "dlsym celt_mode_create failed: %s\n", dlerror());
        Release();
        return EXIT_FAILURE;
    }

    CeltDecoderCreateCustomFunc* celtDecoderCreateCustom = dlsym(handle, "celt_decoder_create_custom");
    if (celtDecoderCreateCustom == NULL) {
        fprintf(stderr, "dlsym celt_decoder_create_custom failed: %s\n", dlerror());
        Release();
        return EXIT_FAILURE;
    }

    celtDecode = dlsym(handle, "celt_decode");
    if (celtDecode == NULL) {
        fprintf(stderr, "dlsym celt_decode failed: %s\n", dlerror());
        Release();
        return EXIT_FAILURE;
    }

    CELTMode *mode = celtModeCreate(SAMPLE_RATE, FRAME_SIZE, NULL);
    if (mode == NULL) {
        fprintf(stderr, "Mode creation failed\n");
        Release();
        return EXIT_FAILURE;
    }

    decoder = celtDecoderCreateCustom(mode, 1, NULL);
    if (decoder == NULL) {
        fprintf(stderr, "Decoder creation failed\n");
        Release();
        return EXIT_FAILURE;
    }

    return EXIT_SUCCESS;
}

int Release() {
    int closed = dlclose(handle);
    if (closed != 0) {
        fprintf(stderr, "Release failed: %s\n", dlerror());
        return EXIT_FAILURE;
    }

    return EXIT_SUCCESS;
}


int Decode(int dataSize, unsigned char *data, const char *destinationPath) {
    size_t outputSize = (dataSize / PACKET_SIZE) * FRAME_SIZE * 2;
    int16_t* output = malloc(outputSize);

    int read = 0;
    int written = 0;

    while (read < dataSize) {
        int result = celtDecode(decoder, data + read, PACKET_SIZE, output + written, FRAME_SIZE);
        if (result < 0) {
            continue;
        }

        read += PACKET_SIZE;
        written += FRAME_SIZE;
    }

    FILE* outputFile = fopen(destinationPath, "wb");
    if (outputFile == NULL) {
        fprintf(stderr, "Unable to open PCM output file: %s\n", destinationPath);
        return EXIT_FAILURE;
    }

    fwrite(output, outputSize, 1, outputFile);
    free(output);
    fclose(outputFile);

    return EXIT_SUCCESS;
}
