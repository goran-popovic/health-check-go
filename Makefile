# Makefile for go-health-check
#
# A Makefile defines named recipes — short aliases for longer commands.
# Run any recipe with: make <recipe-name>  e.g.  make build
#
# PHP parallel: like the "scripts" section in composer.json, but older and
# more universal. Works for any language and is standard in Go projects.
#
# HOW MAKE WORKS:
#   - Each recipe is a target name followed by a colon: build:
#   - The lines below the target (indented with a TAB, not spaces) are
#     the shell commands to run.
#   - .PHONY tells make these are command names, not file names to create.
#     Without it, make gets confused if a file called "build" exists.

.PHONY: build linux mac-arm test race run clean

# The name of the output binary (without extension — make adds .exe on Windows automatically
# when using the go toolchain, but we specify it explicitly for clarity).
BINARY_NAME = health-check

# --- build ---
# Compile a binary for the current OS (Windows → .exe).
#
# -ldflags="-s -w" are linker flags that shrink the binary:
#   -s  strips the symbol table (function names, variable names used by debuggers)
#   -w  strips DWARF debug info (stack traces still work, debugger stepping doesn't)
# Together they reduce binary size by ~30% with no impact on runtime behaviour.
# Only skip them if you need to attach a debugger (rare in production).
#
# In PHP: there's no equivalent — PHP ships source files, not compiled binaries.
# The closest is minifying JS/CSS assets, but that's cosmetic not functional.
build:
	go build -ldflags="-s -w" -o $(BINARY_NAME).exe .

# --- linux ---
# Cross-compile a Linux binary from Windows. No VM, no Docker needed.
#
# This is one of Go's killer features: two environment variables are all
# it takes to target a completely different OS and CPU architecture.
#
# GOOS   = target operating system: linux, windows, darwin (macOS)
# GOARCH = target CPU architecture: amd64 (64-bit Intel/AMD), arm64 (Apple Silicon / ARM servers)
#
# The output binary runs on any Linux amd64 machine — your $5 VPS, a cloud server, a Raspberry Pi 4.
# Copy it, set your .env, and run it. Nothing else to install.
linux:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BINARY_NAME)-linux .

# --- mac-arm ---
# Cross-compile for macOS on Apple Silicon (M1/M2/M3).
mac-arm:
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(BINARY_NAME)-mac .

# --- test ---
# Run all tests across all packages.
test:
	go test ./...

# --- race ---
# Run all tests with the race detector enabled.
# Requires CGO (a C compiler) — see README for setup on Windows.
race:
	PATH="/c/ProgramData/mingw64/mingw64/bin:$$PATH" CGO_ENABLED=1 go test -race ./...

# --- run ---
# Run the app directly without producing a binary file.
# Useful during development — like `php artisan serve`.
run:
	go run .

# --- clean ---
# Delete all compiled binaries from the project directory.
# Like `composer clear-cache` but for build artifacts.
clean:
	rm -f $(BINARY_NAME).exe $(BINARY_NAME)-linux $(BINARY_NAME)-mac health-check-before.exe
