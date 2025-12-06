const std = @import("std");
const builtin = @import("builtin");

// Platform-specific implementations
pub const impl = switch (builtin.os.tag) {
    .linux => @import("linux.zig"),
    .macos => @import("macos.zig"),
    .windows => @import("windows.zig"),
    else => @compileError("Unsupported operating system"),
};

// Common interface that all platform implementations must provide
pub const ProcessInfo = struct {
    pid: u32,
    name: []const u8,
    cmdline: []const u8,
    allocator: std.mem.Allocator,

    pub fn deinit(self: *ProcessInfo) void {
        self.allocator.free(self.name);
        self.allocator.free(self.cmdline);
    }
};

// Find all processes matching the given name
pub fn findProcessesByName(allocator: std.mem.Allocator, name: []const u8) ![]ProcessInfo {
    return try impl.findProcessesByName(allocator, name);
}

// Get process info for a specific PID
pub fn getProcessInfo(allocator: std.mem.Allocator, pid: u32) !ProcessInfo {
    return try impl.getProcessInfo(allocator, pid);
}

// Free a list of ProcessInfo structs
pub fn freeProcessList(processes: []ProcessInfo) void {
    for (processes) |*proc| {
        proc.deinit();
    }
    processes[0].allocator.free(processes);
}

test "process discovery interface" {
    @import("std").testing.refAllDecls(@This());
}
