const std = @import("std");
const os_impl = @import("../os/common.zig");

/// Find a Java process by name and return its PID
/// Returns error.ProcessNotFound if no matching process is found
/// Returns error.MultipleProcesses if multiple processes match
pub fn findJavaProcess(allocator: std.mem.Allocator, name: []const u8) !u32 {
    // Search for processes matching the name
    const processes = try os_impl.findProcessesByName(allocator, name);
    defer os_impl.freeProcessList(processes);

    // Filter for Java processes - store full ProcessInfo for accurate display
    var java_processes = std.ArrayList(os_impl.ProcessInfo).empty;
    defer java_processes.deinit(allocator);

    for (processes) |proc| {
        // Check if it's a Java process
        if (isJavaProcess(proc)) {
            try java_processes.append(allocator, proc);
        }
    }

    if (java_processes.items.len == 0) {
        std.debug.print("No Java process found matching '{s}'\n", .{name});
        return error.ProcessNotFound;
    }

    if (java_processes.items.len > 1) {
        std.debug.print("Multiple Java processes found matching '{s}':\n", .{name});
        for (java_processes.items) |proc| {
            std.debug.print("  PID {d}: {s}\n", .{ proc.pid, proc.cmdline });
        }
        std.debug.print("Please specify --pid explicitly\n", .{});
        return error.MultipleProcesses;
    }

    return java_processes.items[0].pid;
}

/// Check if a process is a Java process
fn isJavaProcess(proc: os_impl.ProcessInfo) bool {
    // Check if process name or command line contains "java"
    if (std.mem.indexOf(u8, proc.name, "java") != null) {
        return true;
    }

    if (std.mem.indexOf(u8, proc.cmdline, "java") != null) {
        return true;
    }

    return false;
}

/// Verify that a PID exists and is a Java process
pub fn verifyJavaProcess(allocator: std.mem.Allocator, pid: u32) !void {
    const proc = try os_impl.getProcessInfo(allocator, pid);
    var mutable_proc = proc;
    defer mutable_proc.deinit();

    if (!isJavaProcess(mutable_proc)) {
        std.debug.print("PID {d} is not a Java process (found: {s})\n", .{ pid, mutable_proc.name });
        return error.NotJavaProcess;
    }
}

test "process discovery" {
    const testing = std.testing;
    const allocator = testing.allocator;

    // Test finding processes (this is platform-dependent, so we can't make strong assertions)
    // Just verify the API works without crashing
    const processes = os_impl.findProcessesByName(allocator, "test") catch |err| {
        // On systems without permissions, skip test
        if (err == error.AccessDenied) {
            return error.SkipZigTest;
        }
        return err;
    };
    defer os_impl.freeProcessList(processes);

    // Should not crash
    try testing.expect(true);
}

test "java process detection" {
    const testing = std.testing;

    // Test isJavaProcess with mock data
    const proc = os_impl.ProcessInfo{
        .pid = 12345,
        .name = "java",
        .cmdline = "/usr/bin/java -jar app.jar",
        .allocator = testing.allocator,
    };

    try testing.expect(isJavaProcess(proc) == true);

    const non_java_proc = os_impl.ProcessInfo{
        .pid = 12346,
        .name = "python",
        .cmdline = "/usr/bin/python script.py",
        .allocator = testing.allocator,
    };

    try testing.expect(isJavaProcess(non_java_proc) == false);
}
