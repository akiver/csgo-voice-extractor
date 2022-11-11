# CSGO voice extractor

CLI to export players' voices from CSGO demos into WAV files.

**Valve Matchmaking demos do not contain voice audio data, hence there is nothing to extract from MM demos.**

## Installation

### Windows

1. Download the last release from [GitHub](https://github.com/akiver/csgo-voice-extractor/releases)
2. Copy/paste the following files from the `CSGO` installation folder next to the executable:

- `tier0.dll`
- `vaudio_celt.dll`

> **Note**  
> By default, these files are located in `C:\Program Files (x86)\Steam\steamapps\common\Counter-Strike Global Offensive\bin`

### macOS

1. Download the last release from [GitHub](https://github.com/akiver/csgo-voice-extractor/releases)
2. Copy/paste the following files from the `CSGO` installation folder next to the executable:

- `libtier0.dylib`
- `vaudio_celt.dylib`

> **Note**  
> By default, these files are located in `~/Applications/Steam/steamapps/common/Counter-Strike Global Offensive/bin/osx64`

### Linux

1. Download the last release from [GitHub](https://github.com/akiver/csgo-voice-extractor/releases)
2. Copy/paste the following files from the `CSGO` installation folder next to the executable:

- `libtier0_client.so`
- `vaudio_celt_client.so`

> **Note**  
> By default, these files are located in `~/.steam/steam/steamapps/common/Counter-Strike Global Offensive/bin`

## Usage

### Windows

```bash
csgove.exe demoPaths... [-output]
```

By default `.dll` files are expected to be in the same directory as the executable.
You can change it by setting the `LD_LIBRARY_PATH` environment variable, example:

```bash
LD_LIBRARY_PATH="C:\Users\username\Desktop" csgove.exe
```

### macOS

> **Warning**  
> The environment variable `DYLD_LIBRARY_PATH` must be set before invoking the program and point to the location of the `.dylib` files!

```bash
DYLD_LIBRARY_PATH=. csgove demoPaths... [-output]
```

### Linux

> **Warning**  
> The environment variable `LD_LIBRARY_PATH` must be set before invoking the program and point to the location of the `.so` files!

```bash
LD_LIBRARY_PATH=. csgove demoPaths... [-output]
```

### Options

`-output <string>`

Folder location where audio files will be written. Current working directory by default.

`-exit-on-first-error`

Stop the program at the first error encountered. By default, the program will continue to the next demo to process if an error occurs.

### Examples

Extract voices from the demo `myDemo.dem` in the current directory:

```bash
csgove myDemo.dem
```

Extract voices from multiple demos using absolute or relative paths:

```bash
csgove myDemo1.dem ../myDemo2.dem "C:\Users\username\Desktop\myDemo3.dem"
```

Change the output location:

```bash
csgove -output "C:\Users\username\Desktop\output" myDemo.dem
```

## Developing

### Requirements

- [Go](https://go.dev/)
- [GCC](https://gcc.gnu.org/)

_Debugging is easier on macOS/Linux **64-bit**, see warnings below._

### Windows

_Because the CSGO audio library is a 32-bit DLL, you need a 32-bit `GCC` and set the Go env variable `GOARCH=386` to build the program._

1. Install `GCC` for Windows, [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) is recommended because it handles both 32-bit and 64-bit when running `go build`.
   If you use [MSYS2](https://www.msys2.org/), it's important to install the 32-bit version (`pacman -S mingw-w64-i686-gcc`).
2. Copy/paste `tier0.dll` and `vaudio_celt.dll` from the CSGO installation folder in the project's root folder.
3. `CGO_ENABLED=1 GOARCH=386 go run .`

> **Warning**  
> Because the Go debugger doesn't support Windows 32-bit and the CSGO lib is a 32-bit DLL, you will not be able to run the Go debugger.  
> If you want to be able to run the debugger for the **Go part only**, you could comment on lines that involve `C/CGO` calls.

### macOS

1. Copy/paste `libtier0.dylib` and `vaudio_celt.dylib` from the CSGO installation folder in the project's root folder.
2. `DYLD_LIBRARY_PATH=. CGO_ENABLED=1 GOARCH=amd64 go run .`

> **Warning**  
> On macOS ARM64, the Go debugger breakpoints will not work because the executable must target amd64 but your OS is ARM64.

### Linux

1. Copy/paste `libtier0_client.so` and `vaudio_celt_client.so` from the CSGO installation folder in the project's root folder.
2. `LD_LIBRARY_PATH=. CGO_ENABLED=1 GOARCH=amd64 go run .`

## Building

### Windows

`make build-windows`

### macOS

`make build-darwin`

### Linux

`make build-linux`

## Credits

Thanks to [@saul](https://github.com/saul) and [@ericek111](https://github.com/ericek111) for their [investigation](https://github.com/saul/demofile/issues/83#issuecomment-1207437098).

## License

[MIT](https://github.com/akiver/csgo-voice-extractor/blob/main/LICENSE)
