const std = @import("std");
const ProcessInfo = @import("common.zig").ProcessInfo;

pub fn findProcessesByName(allocator: std.mem.Allocator, name: []const u8) ![]ProcessInfo {
    var result = std.ArrayList(ProcessInfo).empty;
    errdefer {
        for (result.items) |*proc| {
            proc.deinit();
        }
        result.deinit(allocator);
    }

    // Open /proc directory
    var proc_dir = try std.fs.openDirAbsolute("/proc", .{ .iterate = true });
    defer proc_dir.close();

    var iter = proc_dir.iterate();
    while (try iter.next()) |entry| {
        // Only process numeric directory names (PIDs)
        if (entry.kind != .directory) continue;

        const pid = std.fmt.parseInt(u32, entry.name, 10) catch continue;

        // Try to get process info
        const proc_info = getProcessInfo(allocator, pid) catch continue;

        // Check if process name matches
        if (std.mem.indexOf(u8, proc_info.name, name) != null or
            std.mem.indexOf(u8, proc_info.cmdline, name) != null)
        {
            try result.append(allocator, proc_info);
        } else {
            var mutable_proc = proc_info;
            mutable_proc.deinit();
        }
    }

    return try result.toOwnedSlice(allocator);
}

pub fn getProcessInfo(allocator: std.mem.Allocator, pid: u32) !ProcessInfo {
    // Read /proc/[pid]/stat for process name
    const stat_path = try std.fmt.allocPrint(allocator, "/proc/{d}/stat", .{pid});
    defer allocator.free(stat_path);

    const stat_content = std.fs.readFileAlloc(
        allocator,
        stat_path,
        4096,
    ) catch |err| switch (err) {
        error.FileNotFound => return error.ProcessNotFound,
        else => return err,
    };
    defer allocator.free(stat_content);

    // Parse process name from stat (format: "PID (name) state ...")
    const name_start = std.mem.indexOf(u8, stat_content, "(") orelse return error.InvalidStatFormat;
    const name_end = std.mem.lastIndexOf(u8, stat_content, ")") orelse return error.InvalidStatFormat;

    if (name_end <= name_start + 1) return error.InvalidStatFormat;

    const proc_name = try allocator.dupe(u8, stat_content[name_start + 1 .. name_end]);
    errdefer allocator.free(proc_name);

    // Read /proc/[pid]/cmdline for command line
    const cmdline_path = try std.fmt.allocPrint(allocator, "/proc/{d}/cmdline", .{pid});
    defer allocator.free(cmdline_path);

    const cmdline_content = std.fs.readFileAlloc(
        allocator,
        cmdline_path,
        8192,
    ) catch |err| switch (err) {
        error.FileNotFound => return error.ProcessNotFound,
        else => {
            // If cmdline is unreadable, use empty string
            _ = err; // Suppress unused error
            const empty_cmdline = try allocator.dupe(u8, "");
            return ProcessInfo{
                .pid = pid,
                .name = proc_name,
                .cmdline = empty_cmdline,
                .allocator = allocator,
            };
        },
    };

    // Replace null bytes with spaces for readability
    const cmdline = try allocator.dupe(u8, cmdline_content);
    defer allocator.free(cmdline_content);

    for (cmdline) |*c| {
        if (c.* == 0) c.* = ' ';
    }

    // Trim trailing spaces
    const trimmed = std.mem.trimRight(u8, cmdline, " ");
    const final_cmdline = try allocator.dupe(u8, trimmed);
    allocator.free(cmdline);

    return ProcessInfo{
        .pid = pid,
        .name = proc_name,
        .cmdline = final_cmdline,
        .allocator = allocator,
    };
}

test "linux process discovery" {
    const builtin = @import("builtin");
    // Only run this test on Linux
    if (builtin.os.tag != .linux) return error.SkipZigTest;

    const testing = std.testing;
    const allocator = testing.allocator;

    // Test getting info for PID 1 (init/systemd)
    const proc = try getProcessInfo(allocator, 1);
    var mutable_proc = proc;
    defer mutable_proc.deinit();

    try testing.expect(mutable_proc.pid == 1);
    try testing.expect(mutable_proc.name.len > 0);
}
