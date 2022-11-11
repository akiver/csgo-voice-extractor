// +build windows

#ifndef DLFCN_H
#define DLFCN_H

#define RTLD_LAZY 0x1

// Small wrapper around the Windows API to mimic POSIX dynamic library loading functions.
void *dlopen(const char *filename, int flag);
int dlclose(void *handle);
void *dlsym(void *handle, const char *name);
const char *dlerror();

#endif