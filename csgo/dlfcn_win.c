// +build windows

#include <inttypes.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <windows.h>

static struct LastError {
  long code;
  const char *functionName;
} lastError = {
    0,
    NULL
};

void *dlopen(const char *filename, int flags) {
    HINSTANCE handle = LoadLibraryEx(filename, NULL, LOAD_WITH_ALTERED_SEARCH_PATH);
    if (handle == NULL) {
        lastError.code = GetLastError();
        lastError.functionName = "dlopen";
    }

    return handle;
}

int dlclose(void *handle) {
    BOOL ok = FreeLibrary(handle);
    if (!ok) {
        lastError.code = GetLastError();
        lastError.functionName = "dlclose";
        return -1;
    }

    return 0;
}

void *dlsym(void *handle, const char *name) {
    FARPROC fp = GetProcAddress(handle, name);

    if (fp == NULL) {
        lastError.code = GetLastError();
        lastError.functionName = "dlsym";
    }

    return (void *)(intptr_t)fp;
}

const char *dlerror() {
    static char error[256];

    if (lastError.code) {
        sprintf(error, "%s error #%ld", lastError.functionName, lastError.code);
        return error;
    }

    return NULL;
}