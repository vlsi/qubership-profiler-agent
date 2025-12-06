const std = @import("std");
const process = @import("../actions/process.zig");
const compress = @import("../actions/compress.zig");
const upload = @import("../actions/upload.zig");

pub fn run(allocator: std.mem.Allocator, args: []const []const u8) !void {
    // Simple argument parsing
    var pid: ?u32 = null;
    var name: ?[]const u8 = null;
    var do_thread_dump = true; // Default: enabled
    var do_top_dump = true; // Default: enabled
    var do_compress = false;
    var do_upload = false;
    var show_help = false;

    var i: usize = 0;
    while (i < args.len) : (i += 1) {
        const arg = args[i];

        if (std.mem.eql(u8, arg, "--help") or std.mem.eql(u8, arg, "-h")) {
            show_help = true;
        } else if (std.mem.eql(u8, arg, "--pid") or std.mem.eql(u8, arg, "-p")) {
            i += 1;
            if (i >= args.len) return error.MissingPidValue;
            const pid_value = try std.fmt.parseInt(u32, args[i], 10);
            if (pid_value == 0) {
                std.debug.print("Error: PID cannot be 0\n", .{});
                return error.InvalidPid;
            }
            pid = pid_value;
        } else if (std.mem.eql(u8, arg, "--name") or std.mem.eql(u8, arg, "-n")) {
            i += 1;
            if (i >= args.len) return error.MissingNameValue;
            name = args[i];
        } else if (std.mem.eql(u8, arg, "--no-thread-dump")) {
            do_thread_dump = false;
        } else if (std.mem.eql(u8, arg, "--no-top-dump")) {
            do_top_dump = false;
        } else if (std.mem.eql(u8, arg, "--compress") or std.mem.eql(u8, arg, "-c")) {
            do_compress = true;
        } else if (std.mem.eql(u8, arg, "--upload") or std.mem.eql(u8, arg, "-u")) {
            do_upload = true;
        }
    }

    if (show_help) {
        try printHelp();
        return;
    }

    // Determine the target PID
    const target_pid = if (pid) |p|
        p
    else if (name) |n|
        try process.findJavaProcess(allocator, n)
    else if (std.posix.getenv("JAVA_PROCESS_NAME")) |env_name|
        try process.findJavaProcess(allocator, env_name)
    else {
        std.debug.print("Error: Must specify either --pid, --name, or JAVA_PROCESS_NAME environment variable\n", .{});
        return error.MissingPid;
    };

    std.debug.print("Collecting dumps for PID {d}...\n", .{target_pid});

    // Track files to compress/upload
    var files_to_process = std.ArrayList([]const u8).empty;
    defer {
        for (files_to_process.items) |file| {
            allocator.free(file);
        }
        files_to_process.deinit(allocator);
    }

    // Collect thread dump if enabled
    if (do_thread_dump) {
        std.debug.print("Collecting thread dump...\n", .{});
        const thread_dump_file = try collectThreadDump(allocator, target_pid);
        try files_to_process.append(allocator, thread_dump_file);
        std.debug.print("Thread dump saved to: {s}\n", .{thread_dump_file});
    }

    // Collect top dump if enabled
    if (do_top_dump) {
        std.debug.print("Collecting CPU usage (top)...\n", .{});
        const top_dump_file = try collectTopDump(allocator, target_pid);
        try files_to_process.append(allocator, top_dump_file);
        std.debug.print("Top dump saved to: {s}\n", .{top_dump_file});
    }

    // Process files (compress and/or upload)
    for (files_to_process.items) |dump_file| {
        var file_to_upload: []const u8 = dump_file;
        var should_free_upload_path = false;
        defer if (should_free_upload_path) allocator.free(file_to_upload);

        // Compress if requested
        if (do_compress) {
            std.debug.print("Compressing {s}...\n", .{dump_file});
            const compressed_path = try compress.compressAndDeleteSource(allocator, dump_file, null);
            file_to_upload = compressed_path;
            should_free_upload_path = true;
        }

        // Upload if requested
        if (do_upload) {
            try upload.uploadFileFromEnv(allocator, file_to_upload);
        }
    }

    std.debug.print("Dump collection complete\n", .{});
}

fn printHelp() !void {
    std.debug.print(
        \\diagtools dump - Collect Java thread dump and CPU usage
        \\
        \\Usage: diagtools dump [options]
        \\
        \\Options:
        \\  -h, --help              Show this help message
        \\  -p, --pid <PID>         Target process ID
        \\  -n, --name <NAME>       Target process name (finds Java process)
        \\  -c, --compress          Compress dumps to ZIP
        \\  -u, --upload            Upload to diagnostic collector service
        \\  --no-thread-dump        Skip thread dump collection
        \\  --no-top-dump           Skip CPU usage (top) collection
        \\
        \\Examples:
        \\  diagtools dump --pid 12345
        \\  diagtools dump --name myapp --compress --upload
        \\  diagtools dump --name myapp --no-top-dump
        \\
        \\Environment Variables:
        \\  JAVA_PROCESS_NAME       Default process name if --name not specified
        \\  DIAGCOLLECTOR_URL       URL for upload (required with --upload)
        \\  DIAGCOLLECTOR_TOKEN     Auth token for upload
        \\
    , .{});
}

fn generateDumpPath(allocator: std.mem.Allocator, pid: u32, suffix: []const u8) ![]u8 {
    const timestamp = std.time.timestamp();
    const datetime = std.time.epoch.EpochSeconds{ .secs = @intCast(timestamp) };
    const day_seconds = datetime.getDaySeconds();
    const year_day = datetime.getEpochDay().calculateYearDay();
    const month_day = year_day.calculateMonthDay();

    const hours = day_seconds.getHoursIntoDay();
    const minutes = day_seconds.getMinutesIntoHour();
    const seconds = day_seconds.getSecondsIntoMinute();

    return try std.fmt.allocPrint(
        allocator,
        "{d:0>4}-{d:0>2}-{d:0>2}T{d:0>2}-{d:0>2}-{d:0>2}-{d}{s}",
        .{ year_day.year, month_day.month.numeric(), month_day.day_index + 1, hours, minutes, seconds, pid, suffix },
    );
}

fn collectThreadDump(allocator: std.mem.Allocator, pid: u32) ![]u8 {
    const output_file = try generateDumpPath(allocator, pid, ".td.txt");
    errdefer allocator.free(output_file);

    // Execute jstack to collect thread dump
    const pid_str = try std.fmt.allocPrint(allocator, "{d}", .{pid});
    defer allocator.free(pid_str);

    var argv = std.ArrayList([]const u8).empty;
    defer argv.deinit(allocator);

    try argv.append(allocator, "jstack");
    try argv.append(allocator, pid_str);

    // Execute jstack command and capture output
    var child = std.process.Child.init(argv.items, allocator);
    child.stdout_behavior = .Pipe;
    child.stderr_behavior = .Pipe;

    try child.spawn();

    // Read stdout
    const output = try child.stdout.?.readToEndAlloc(allocator, 10 * 1024 * 1024); // Max 10MB
    defer allocator.free(output);

    // Read stderr (for logging)
    const stderr_output = try child.stderr.?.readToEndAlloc(allocator, 1024 * 1024); // Max 1MB
    defer allocator.free(stderr_output);

    const term = try child.wait();

    switch (term) {
        .Exited => |code| {
            if (code != 0) {
                std.debug.print("jstack exited with code {d}\n", .{code});
                if (stderr_output.len > 0) {
                    std.debug.print("jstack stderr: {s}\n", .{stderr_output});
                }
                return error.JstackFailed;
            }
        },
        else => {
            std.debug.print("jstack terminated abnormally\n", .{});
            return error.JstackFailed;
        },
    }

    // Write output to file
    const file = try std.fs.cwd().createFile(output_file, .{});
    defer file.close();
    try file.writeAll(output);

    return output_file;
}

fn collectTopDump(allocator: std.mem.Allocator, pid: u32) ![]u8 {
    const output_file = try generateDumpPath(allocator, pid, ".top.txt");
    errdefer allocator.free(output_file);

    // Execute top to collect CPU usage
    // top -Hb -p<pid> -oTIME+ -d60 -n1
    const pid_arg = try std.fmt.allocPrint(allocator, "-p{d}", .{pid});
    defer allocator.free(pid_arg);

    var argv = std.ArrayList([]const u8).empty;
    defer argv.deinit(allocator);

    try argv.append(allocator, "top");
    try argv.append(allocator, "-Hb"); // Batch mode with threads
    try argv.append(allocator, pid_arg);
    try argv.append(allocator, "-oTIME+"); // Sort by CPU time
    try argv.append(allocator, "-d60"); // Delay (not really used with -n1)
    try argv.append(allocator, "-n1"); // Only one iteration

    // Execute top command and capture output
    var child = std.process.Child.init(argv.items, allocator);
    child.stdout_behavior = .Pipe;
    child.stderr_behavior = .Pipe;

    try child.spawn();

    // Read stdout
    const output = try child.stdout.?.readToEndAlloc(allocator, 10 * 1024 * 1024); // Max 10MB
    defer allocator.free(output);

    // Read stderr (for logging)
    const stderr_output = try child.stderr.?.readToEndAlloc(allocator, 1024 * 1024); // Max 1MB
    defer allocator.free(stderr_output);

    const term = try child.wait();

    switch (term) {
        .Exited => |code| {
            if (code != 0) {
                std.debug.print("top exited with code {d}\n", .{code});
                if (stderr_output.len > 0) {
                    std.debug.print("top stderr: {s}\n", .{stderr_output});
                }
                return error.TopFailed;
            }
        },
        else => {
            std.debug.print("top terminated abnormally\n", .{});
            return error.TopFailed;
        },
    }

    // Write output to file
    const file = try std.fs.cwd().createFile(output_file, .{});
    defer file.close();
    try file.writeAll(output);

    return output_file;
}

test "dump command tests" {
    @import("std").testing.refAllDecls(@This());
}
