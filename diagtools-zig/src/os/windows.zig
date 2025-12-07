const std = @import("std");
const ProcessInfo = @import("common.zig").ProcessInfo;
const windows = std.os.windows;

// Windows-specific imports
const PROCESSENTRY32 = extern struct {
    dwSize: windows.DWORD,
    cntUsage: windows.DWORD,
    th32ProcessID: windows.DWORD,
    th32DefaultHeapID: usize,
    th32ModuleID: windows.DWORD,
    cntThreads: windows.DWORD,
    th32ParentProcessID: windows.DWORD,
    pcPriClassBase: windows.LONG,
    dwFlags: windows.DWORD,
    szExeFile: [windows.MAX_PATH]u8,
};

const TH32CS_SNAPPROCESS: windows.DWORD = 0x00000002;

extern "kernel32" fn CreateToolhelp32Snapshot(
    dwFlags: windows.DWORD,
    th32ProcessID: windows.DWORD,
) callconv(windows.WINAPI) ?windows.HANDLE;

extern "kernel32" fn Process32First(
    hSnapshot: windows.HANDLE,
    lppe: *PROCESSENTRY32,
) callconv(windows.WINAPI) windows.BOOL;

extern "kernel32" fn Process32Next(
    hSnapshot: windows.HANDLE,
    lppe: *PROCESSENTRY32,
) callconv(windows.WINAPI) windows.BOOL;

extern "kernel32" fn OpenProcess(
    dwDesiredAccess: windows.DWORD,
    bInheritHandle: windows.BOOL,
    dwProcessId: windows.DWORD,
) callconv(windows.WINAPI) ?windows.HANDLE;

extern "kernel32" fn QueryFullProcessImageNameA(
    hProcess: windows.HANDLE,
    dwFlags: windows.DWORD,
    lpExeName: [*]u8,
    lpdwSize: *windows.DWORD,
) callconv(windows.WINAPI) windows.BOOL;

const PROCESS_QUERY_LIMITED_INFORMATION: windows.DWORD = 0x1000;

pub fn findProcessesByName(allocator: std.mem.Allocator, name: []const u8) ![]ProcessInfo {
    var result = std.ArrayList(ProcessInfo).empty;
    errdefer {
        for (result.items) |*proc| {
            proc.deinit();
        }
        result.deinit(allocator);
    }

    // Create snapshot of all processes
    const snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0) orelse
        return error.SnapshotFailed;
    defer windows.CloseHandle(snapshot);

    var entry: PROCESSENTRY32 = undefined;
    entry.dwSize = @sizeOf(PROCESSENTRY32);

    // Get first process
    if (Process32First(snapshot, &entry) == 0) {
        return error.SnapshotFailed;
    }

    // Iterate through processes
    while (true) {
        const pid = entry.th32ProcessID;
        const exe_len = std.mem.indexOfScalar(u8, &entry.szExeFile, 0) orelse entry.szExeFile.len;
        const exe_name = entry.szExeFile[0..exe_len];

        // Check if process name matches
        if (std.mem.indexOf(u8, exe_name, name) != null) {
            const proc_info = getProcessInfo(allocator, pid) catch {
                // If we can't get full info, create basic info from snapshot
                const proc_name = try allocator.dupe(u8, exe_name);
                errdefer allocator.free(proc_name);

                const cmdline = try allocator.dupe(u8, exe_name);

                try result.append(allocator, ProcessInfo{
                    .pid = pid,
                    .name = proc_name,
                    .cmdline = cmdline,
                    .allocator = allocator,
                });
                continue;
            };

            try result.append(allocator, proc_info);
        }

        // Get next process
        if (Process32Next(snapshot, &entry) == 0) {
            break;
        }
    }

    return try result.toOwnedSlice(allocator);
}

pub fn getProcessInfo(allocator: std.mem.Allocator, pid: u32) !ProcessInfo {
    // Open process handle
    const handle = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, 0, pid) orelse
        return error.ProcessNotFound;
    defer windows.CloseHandle(handle);

    // Get full process path
    var path_buf: [windows.MAX_PATH]u8 = undefined;
    var path_size: windows.DWORD = windows.MAX_PATH;

    const success = QueryFullProcessImageNameA(handle, 0, &path_buf, &path_size);
    if (success == 0) {
        return error.QueryFailed;
    }

    const full_path = path_buf[0..path_size];

    // Extract just the filename from the path
    const name = if (std.mem.lastIndexOfScalar(u8, full_path, '\\')) |idx|
        full_path[idx + 1 ..]
    else
        full_path;

    const proc_name = try allocator.dupe(u8, name);
    errdefer allocator.free(proc_name);

    const cmdline = try allocator.dupe(u8, full_path);

    return ProcessInfo{
        .pid = pid,
        .name = proc_name,
        .cmdline = cmdline,
        .allocator = allocator,
    };
}

test "windows process discovery" {
    const builtin = @import("builtin");
    // Only run this test on Windows
    if (builtin.os.tag != .windows) return error.SkipZigTest;

    const testing = std.testing;
    const allocator = testing.allocator;

    // Test getting info for current process
    const current_pid = windows.kernel32.GetCurrentProcessId();
    const proc = try getProcessInfo(allocator, current_pid);
    var mutable_proc = proc;
    defer mutable_proc.deinit();

    try testing.expect(mutable_proc.pid == current_pid);
    try testing.expect(mutable_proc.name.len > 0);
}
