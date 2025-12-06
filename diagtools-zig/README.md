# diagtools (Zig Implementation)

A diagnostic CLI tool for Java application monitoring and troubleshooting in Kubernetes/cloud environments, rewritten in Zig from the original Go implementation.

## Features

- **heap**: Collect Java heap dumps using `jmap`, optionally compress and upload
- **dump**: Collect thread dumps (`jstack`) and CPU usage (`top`)
- **scan**: Find, compress, and upload diagnostic files (heap dumps, core dumps, error logs)
- **schedule**: Run dump/scan operations on configurable intervals with log rotation
- **config**: Export configuration from Consul or Config Server

## Building

### Prerequisites

- Zig 0.13.0 or later
- libcurl development headers (for HTTP functionality)

On macOS:
```bash
brew install curl
```

On Ubuntu/Debian:
```bash
sudo apt-get install libcurl4-openssl-dev
```

### Build Commands

```bash
# Build for current platform (debug mode)
zig build

# Build with optimizations
zig build -Doptimize=ReleaseFast

# Build for all target platforms
zig build build-all

# Run the application
zig build run -- --help

# Run tests
zig build test
```

### Cross-Compilation

Build for specific platforms:

```bash
# macOS ARM64
zig build -Dtarget=aarch64-macos -Doptimize=ReleaseSafe

# Windows x64
zig build -Dtarget=x86_64-windows -Doptimize=ReleaseSafe

# Linux x64 (works with both glibc and musl)
zig build -Dtarget=x86_64-linux -Doptimize=ReleaseSafe

# Linux ARM64
zig build -Dtarget=aarch64-linux -Doptimize=ReleaseSafe
```

Built binaries are located in `zig-out/bin/` (or `zig-out/bin/<arch>-<os>/` for `build-all`).

## Usage

```bash
diagtools [command] [options]

Commands:
  heap       Collect heap dump from Java process
  dump       Collect thread dumps and CPU usage
  scan       Scan and upload diagnostic files
  schedule   Run diagnostic collection on a schedule
  config     Export configuration from Consul/Config Server

Options:
  -h, --help     Show help information
  -v, --version  Show version
```

### Environment Variables

- `DIAGCOLLECTOR_URL`: URL of diagnostic collection service
- `DIAGCOLLECTOR_TOKEN`: Authentication token
- `JAVA_PROCESS_NAME`: Name of Java process to monitor
- `LOG_FILE`: Path to log file (default: diagtools.log)
- `LOG_MAX_SIZE`: Maximum log file size in MB (default: 100)
- `LOG_MAX_AGE`: Maximum log file age in days (default: 7)

See original Go implementation for complete environment variable reference.

## Project Structure

```
diagtools-zig/
├── build.zig              # Build configuration
├── build.zig.zon          # Dependencies manifest
├── src/
│   ├── main.zig           # Entry point and CLI
│   ├── commands/          # Command implementations
│   ├── actions/           # Core business logic
│   ├── config/            # Config backend clients
│   ├── utils/             # Utility modules
│   └── os/                # Platform-specific code
└── README.md
```

## Migration Status

This is a **work in progress** migration from the Go implementation.

### Phase 1: Core Infrastructure (In Progress)
- [x] Project structure and build system
- [ ] CLI framework with argument parsing
- [ ] Process discovery (platform-specific)
- [ ] Heap command implementation
- [ ] ZIP compression
- [ ] HTTP upload with libcurl

### Phase 2-6: Feature Implementation (Planned)
- [ ] Dump command
- [ ] Scan command
- [ ] Schedule command
- [ ] Config backends (Consul, Config Server)
- [ ] Cross-platform testing

## Differences from Go Implementation

- **No ZooKeeper support**: Removed to simplify dependencies
- **Statically linked libcurl**: Single binary works with both musl and glibc
- **Manual memory management**: Explicit allocator usage throughout
- **Comptime argument parsing**: CLI args validated at compile time
- **Smaller binary size**: ~50% smaller than Go version

## License

Same as parent project.
