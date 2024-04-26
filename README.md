# Counter-Strike voice extractor

CLI to export players' voices from CSGO/CS2 demos into WAV files.

> [!WARNING]  
> **Valve Matchmaking demos do not contain voice audio data, hence there is nothing to extract from MM demos.**

## Installation

Download the last release for your OS from [GitHub](https://github.com/akiver/csgo-voice-extractor/releases/latest).

## Usage

### Windows

```bash
csgove.exe demoPaths... [-output]
```

By default `.dll` files are expected to be in the same directory as the executable.
You can change it by setting the `LD_LIBRARY_PATH` environment variable. Example:

```bash
LD_LIBRARY_PATH="C:\Users\username\Desktop" csgove.exe
```

### macOS

> [!CAUTION]  
> The environment variable `DYLD_LIBRARY_PATH` must be set before invoking the program and point to the location of the `.dylib` files!

```bash
DYLD_LIBRARY_PATH=. csgove demoPaths... [-output]
```

### Linux

> [!CAUTION]  
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
- [Chocolatey](https://chocolatey.org/) (Windows only)

_Debugging is easier on macOS/Linux **64-bit**, see warnings below._

### Windows

_Because the CSGO audio library is a 32-bit DLL, you need a 32-bit `GCC` and set the Go env variable `GOARCH=386` to build the program._

> [!IMPORTANT]  
> Use a unix like shell such as [Git Bash](https://git-scm.com/), it will not work with `cmd.exe`!

> [!WARNING]  
> The `$GCC_PATH` variable in the following steps is the path where `gcc.exe` is located.  
> By default, it's `C:\TDM-GCC-64\bin` when using [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) (highly recommended).

1. Install `GCC` for Windows, [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) is recommended because it handles both 32-bit and 64-bit when running `go build`.
   If you use [MSYS2](https://www.msys2.org/), it's important to install the 32-bit version (`pacman -S mingw-w64-i686-gcc`).
2. Install `pkg-config` using [chocolatey](https://chocolatey.org/) by running `choco install pkgconfiglite`.  
   It's **highly recommended** to use `choco` otherwise you would have to build `pkg-config` and copy/paste the `pkg-config.exe` binary in your `$GCC_PATH`.
3. Download the source code of [Opus](https://opus-codec.org/downloads/)
4. Extract the archive, rename the folder to `opus` and place it in the project's root folder
5. Open the `opus/win32/VS2015/opus.sln` file with Visual Studio (upgrade the project if asked)
6. Build the `Release` configuration for `Win32` (**not `x64`** - it's important to build the 32-bit version!)
7. Copy/paste the `opus.dll` file in `$GCC_PATH` and `dist/bin/win32-x64`
8. Copy/paste the C header files located inside the `include` folder file in `$GCC_PATH\include\opus` (create the folders if needed)
9. Copy/paste the `opus.pc.example` to `opus.pc` file and edit the `prefix` variable to match your `GCC` installation path **if necessary**.
10. `PKG_CONFIG_PATH=$(realpath .) LD_LIBRARY_PATH=dist/bin/win32-x64 CGO_ENABLED=1 GOARCH=386 go run -tags nolibopusfile .`

> [!WARNING]  
> Because the Go debugger doesn't support Windows 32-bit and the CSGO lib is a 32-bit DLL, you will not be able to run the Go debugger.  
> If you want to be able to run the debugger for the **Go part only**, you could comment on lines that involve `C/CGO` calls.

### macOS

> [!IMPORTANT]  
> On macOS `ARM64`, the `x64` version of Homebrew must be installed!  
> You can install it by adding `arch -x86_64` before the official command to install Homebrew (`arch -x86_64 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`)

1. Install [Homebrew](https://brew.sh) **x64 version**
2. `arch -x86_64 brew install opus`
3. `arch -x86_64 brew install pkg-config`
4. `cp /usr/local/Cellar/opus/1.4/lib/libopus.0.dylib dist/bin/darwin-x64` (`arch -x86_64 brew info opus` to get the path)
5. `DYLD_LIBRARY_PATH=dist/bin/darwin-x64 CGO_ENABLED=1 GOARCH=amd64 go run -tags nolibopusfile .`

> [!WARNING]  
> On macOS ARM64, the Go debugger breakpoints will not work because the executable must target amd64 but your OS is ARM64.

### Linux

1. `sudo apt install pkg-config libopus-dev`
2. `cp /usr/lib/x86_64-linux-gnu/libopus.so.0 dist/bin/linux-x64` (you may need to change the path depending on your distro)
3. `LD_LIBRARY_PATH=dist/bin/linux-x64 CGO_ENABLED=1 GOARCH=amd64 go run -tags nolibopusfile .`

## Building

### Windows

`make build-windows`

### macOS

`make build-darwin`

### Linux

`make build-linux`

## Credits

Thanks to [@saul](https://github.com/saul) and [@ericek111](https://github.com/ericek111) for their [CSGO investigation](https://github.com/saul/demofile/issues/83#issuecomment-1207437098).  
Thanks to [@DandrewsDev](https://github.com/DandrewsDev) for his work on [CS2 voice data extraction](https://github.com/DandrewsDev/CS2VoiceData).

## License

[MIT](https://github.com/akiver/csgo-voice-extractor/blob/main/LICENSE)
