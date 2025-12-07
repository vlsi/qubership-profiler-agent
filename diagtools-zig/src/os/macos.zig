const std = @import("std");
const ProcessInfo = @import("common.zig").ProcessInfo;
const c = @cImport({
    @cInclude("sys/sysctl.h");
    @cInclude("sys/proc.h");
    @cInclude("libproc.h");
});

pub fn findProcessesByName(allocator: std.mem.Allocator, name: []const u8) ![]ProcessInfo {
    var result = std.ArrayList(ProcessInfo).empty;
    errdefer {
        for (result.items) |*proc| {
            proc.deinit();
        }
        result.deinit(allocator);
    }

    // Get list of all process IDs using sysctl
    var mib = [_]c_int{ c.CTL_KERN, c.KERN_PROC, c.KERN_PROC_ALL, 0 };
    var size: usize = 0;

    // First call to get size
    if (c.sysctl(&mib, 4, null, &size, null, 0) == -1) {
        return error.SysctlFailed;
    }

    // Allocate buffer for process list
    const proc_count = size / @sizeOf(c.kinfo_proc);
    const procs = try allocator.alloc(c.kinfo_proc, proc_count);
    defer allocator.free(procs);

    // Second call to get actual data
    if (c.sysctl(&mib, 4, procs.ptr, &size, null, 0) == -1) {
        return error.SysctlFailed;
    }

    const actual_count = size / @sizeOf(c.kinfo_proc);

    // Iterate through processes
    for (procs[0..actual_count]) |proc| {
        const pid: u32 = @intCast(proc.kp_proc.p_pid);
        if (pid == 0) continue;

        // Try to get full process info
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
    // Get process name from sysctl
    var mib = [_]c_int{ c.CTL_KERN, c.KERN_PROC, c.KERN_PROC_PID, @intCast(pid) };
    var proc: c.kinfo_proc = undefined;
    var size: usize = @sizeOf(c.kinfo_proc);

    if (c.sysctl(&mib, 4, &proc, &size, null, 0) == -1) {
        return error.ProcessNotFound;
    }

    // Get process name (from kinfo_proc structure)
    const proc_name_cstr = &proc.kp_proc.p_comm;
    const proc_name_len = std.mem.indexOfScalar(u8, proc_name_cstr, 0) orelse proc_name_cstr.len;
    const proc_name = try allocator.dupe(u8, proc_name_cstr[0..proc_name_len]);
    errdefer allocator.free(proc_name);

    // Get command line arguments using proc_pidpath and proc_pidinfo
    var pathbuf: [c.PROC_PIDPATHINFO_MAXSIZE]u8 = undefined;
    const ret = c.proc_pidpath(@intCast(pid), &pathbuf, c.PROC_PIDPATHINFO_MAXSIZE);

    const cmdline = if (ret > 0) blk: {
        const path_len: usize = @intCast(ret);
        break :blk try allocator.dupe(u8, pathbuf[0..path_len]);
    } else blk: {
        // Fallback: try to get command line via sysctl KERN_PROCARGS2
        var args_mib = [_]c_int{ c.CTL_KERN, c.KERN_PROCARGS2, @intCast(pid) };
        var args_size: usize = 0;

        // Get size
        if (c.sysctl(&args_mib, 3, null, &args_size, null, 0) == -1) {
            break :blk try allocator.dupe(u8, "");
        }

        const args_buf = try allocator.alloc(u8, args_size);
        defer allocator.free(args_buf);

        // Get data
        if (c.sysctl(&args_mib, 3, args_buf.ptr, &args_size, null, 0) == -1) {
            break :blk try allocator.dupe(u8, "");
        }

        // Parse KERN_PROCARGS2 format
        // Format: argc (int) + executable path (null-terminated) + null bytes + args
        if (args_size < 4) {
            break :blk try allocator.dupe(u8, "");
        }

        // Skip argc and find the executable path
        var offset: usize = 4;
        while (offset < args_size and args_buf[offset] != 0) : (offset += 1) {}

        // Skip null bytes
        while (offset < args_size and args_buf[offset] == 0) : (offset += 1) {}

        if (offset >= args_size) {
            break :blk try allocator.dupe(u8, "");
        }

        // Get arguments (replace null bytes with spaces)
        const args_start = offset;
        const args_data = args_buf[args_start..args_size];

        const cmdline_buf = try allocator.alloc(u8, args_data.len);
        @memcpy(cmdline_buf, args_data);

        for (cmdline_buf) |*c_byte| {
            if (c_byte.* == 0) c_byte.* = ' ';
        }

        const trimmed = std.mem.trimRight(u8, cmdline_buf, " ");
        const result = try allocator.dupe(u8, trimmed);
        allocator.free(cmdline_buf);

        break :blk result;
    };

    return ProcessInfo{
        .pid = pid,
        .name = proc_name,
        .cmdline = cmdline,
        .allocator = allocator,
    };
}

test "macos process discovery" {
    const builtin = @import("builtin");
    // Only run this test on macOS
    if (builtin.os.tag != .macos) return error.SkipZigTest;

    const testing = std.testing;
    const allocator = testing.allocator;

    // Test getting info for PID 1 (launchd)
    const proc = try getProcessInfo(allocator, 1);
    var mutable_proc = proc;
    defer mutable_proc.deinit();

    try testing.expect(mutable_proc.pid == 1);
    try testing.expect(mutable_proc.name.len > 0);
}
