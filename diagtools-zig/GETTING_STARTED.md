# Getting Started with diagtools (Zig)

This guide will help you get started with developing and using the Zig version of diagtools.

## Prerequisites

### 1. Install Zig

You need Zig 0.13.0 or later.

**macOS (Homebrew):**
```bash
brew install zig
```

**macOS (Direct Download):**
```bash
# Download from https://ziglang.org/download/
# Extract and add to PATH
```

**Linux:**
```bash
# Download from https://ziglang.org/download/
wget https://ziglang.org/download/0.13.0/zig-linux-x86_64-0.13.0.tar.xz
tar -xf zig-linux-x86_64-0.13.0.tar.xz
sudo mv zig-linux-x86_64-0.13.0 /opt/zig
export PATH=/opt/zig:$PATH
```

**Windows:**
```powershell
# Download from https://ziglang.org/download/
# Extract and add to PATH
```

Verify installation:
```bash
zig version
# Should output: 0.13.0 or later
```

### 2. Install libcurl Development Headers

diagtools uses libcurl for HTTP upload functionality.

**macOS:**
```bash
brew install curl
```

**Ubuntu/Debian:**
```bash
sudo apt-get install libcurl4-openssl-dev
```

**Fedora/RHEL:**
```bash
sudo dnf install libcurl-devel
```

**Windows:**
libcurl is typically included with Zig's libc on Windows, but you may need to install it separately.

## Building diagtools

### Development Build (Debug)

```bash
cd diagtools-zig
zig build
```

This creates a debug binary at `zig-out/bin/diagtools`.

### Release Build

For production use, build with optimizations:

```bash
# Optimized for speed
zig build -Doptimize=ReleaseFast

# Optimized for size
zig build -Doptimize=ReleaseSmall

# Optimized with safety checks (recommended)
zig build -Doptimize=ReleaseSafe
```

### Cross-Compilation

Build for specific platforms:

```bash
# macOS ARM64
zig build -Dtarget=aarch64-macos -Doptimize=ReleaseSafe

# Windows x64
zig build -Dtarget=x86_64-windows -Doptimize=ReleaseSafe

# Linux x64
zig build -Dtarget=x86_64-linux -Doptimize=ReleaseSafe

# Linux ARM64
zig build -Dtarget=aarch64-linux -Doptimize=ReleaseSafe
```

### Build for All Platforms

```bash
zig build build-all
```

This creates binaries for all supported platforms in:
- `zig-out/bin/aarch64-macos/diagtools`
- `zig-out/bin/x86_64-windows/diagtools.exe`
- `zig-out/bin/x86_64-linux/diagtools`
- `zig-out/bin/aarch64-linux/diagtools`

## Running diagtools

### Run Directly with zig build

```bash
zig build run -- --help
zig build run -- heap --help
zig build run -- heap --name myapp
```

### Run the Binary

```bash
./zig-out/bin/diagtools --help
./zig-out/bin/diagtools heap --name myapp --compress --upload
```

## Testing

### Run All Tests

```bash
zig build test
```

### Run Tests for Specific Module

```bash
zig test src/actions/process.zig
zig test src/actions/compress.zig
```

### Test with Verbose Output

```bash
zig build test --summary all
```

## Development Workflow

### 1. Make Changes

Edit files in `src/`:
- `src/main.zig` - Main entry point
- `src/commands/*.zig` - Command implementations
- `src/actions/*.zig` - Core business logic
- `src/os/*.zig` - Platform-specific code

### 2. Format Code

```bash
zig fmt src/
```

### 3. Build and Test

```bash
zig build
zig build test
```

### 4. Run

```bash
zig build run -- heap --name java
```

## IDE Setup

### VS Code (Recommended)

1. Install the **Zig Language** extension (ziglang.vscode-zig)
2. The extension automatically installs ZLS (Zig Language Server)
3. You'll get:
   - Autocomplete
   - Go to definition
   - Error checking
   - Integrated debugging

### Neovim

Use ZLS with your LSP client:

```lua
require'lspconfig'.zls.setup{}
```

### Debugging

**VS Code:**
1. Install CodeLLDB extension
2. Use the following `launch.json`:

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "type": "lldb",
            "request": "launch",
            "name": "Debug diagtools",
            "program": "${workspaceFolder}/zig-out/bin/diagtools",
            "args": ["heap", "--help"],
            "cwd": "${workspaceFolder}",
            "preLaunchTask": "zig build"
        }
    ]
}
```

**Command Line (LLDB):**

```bash
zig build
lldb ./zig-out/bin/diagtools
(lldb) run heap --help
```

**Command Line (GDB on Linux):**

```bash
zig build
gdb ./zig-out/bin/diagtools
(gdb) run heap --help
```

## Usage Examples

### Collect Heap Dump

```bash
# By process name
diagtools heap --name myapp

# By PID
diagtools heap --pid 12345

# With compression
diagtools heap --name myapp --compress

# With upload
export DIAGCOLLECTOR_URL=https://collector.example.com/upload
export DIAGCOLLECTOR_TOKEN=your-token-here
diagtools heap --name myapp --compress --upload

# Live heap dump only
diagtools heap --name myapp --live
```

### Environment Variables

```bash
# Set up environment
export JAVA_PROCESS_NAME=myapp
export DIAGCOLLECTOR_URL=https://collector.example.com/upload
export DIAGCOLLECTOR_TOKEN=your-auth-token
export LOG_FILE=/var/log/diagtools.log
export LOG_MAX_SIZE=100  # MB
export LOG_MAX_AGE=7     # days

# Now you can run without flags
diagtools heap --compress --upload
```

## Common Issues

### "zig: command not found"

Zig is not in your PATH. Add it:

```bash
export PATH=/path/to/zig:$PATH
```

### "curl/curl.h: No such file or directory"

libcurl development headers are not installed. See Prerequisites section.

### "Process not found"

Make sure:
1. The Java process is running
2. You have permission to access process information
3. The process name matches (case-sensitive)

### Memory Leak Detected

This is a debug-mode warning. If you see this during development:
1. Check that all `defer allocator.free()` are in place
2. Use `defer obj.deinit()` for cleanup
3. Run tests to catch leaks: `zig build test`

In release builds, this warning is not shown.

## Next Steps

- Read [README.md](README.md) for full feature documentation
- Explore the [source code](src/) to understand the architecture
- Check out the [Go implementation](../diagtools/) for reference
- Add new commands (dump, scan, schedule) following the heap command pattern

## Getting Help

- **Zig Language Documentation**: https://ziglang.org/documentation/master/
- **Zig Guide**: https://zig.guide/
- **Zig Community Forum**: https://ziggit.dev/
- **Zig Discord**: https://discord.gg/zig

## Contributing

When contributing:

1. Run `zig fmt` on all changed files
2. Ensure `zig build test` passes
3. Add tests for new functionality
4. Update documentation as needed
5. Follow existing code patterns for consistency
