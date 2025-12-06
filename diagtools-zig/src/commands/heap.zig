const std = @import("std");
// TODO: Re-enable when zig-clap is compatible with Zig 0.15.2
// const clap = @import("clap");
const process = @import("../actions/process.zig");
const compress = @import("../actions/compress.zig");
const upload = @import("../actions/upload.zig");

pub fn run(allocator: std.mem.Allocator, args: []const []const u8) !void {
    // Simple argument parsing
    var pid: ?u32 = null;
    var name: ?[]const u8 = null;
    var file: ?[]const u8 = null;
    var do_compress = false;
    var do_upload = false;
    var live = false;
    var show_help = false;

    var i: usize = 0;
    while (i < args.len) : (i += 1) {
        const arg = args[i];

        if (std.mem.eql(u8, arg, "--help") or std.mem.eql(u8, arg, "-h")) {
            show_help = true;
        } else if (std.mem.eql(u8, arg, "--pid") or std.mem.eql(u8, arg, "-p")) {
            i += 1;
            if (i >= args.len) return error.MissingPidValue;
            pid = try std.fmt.parseInt(u32, args[i], 10);
        } else if (std.mem.eql(u8, arg, "--name") or std.mem.eql(u8, arg, "-n")) {
            i += 1;
            if (i >= args.len) return error.MissingNameValue;
            name = args[i];
        } else if (std.mem.eql(u8, arg, "--file") or std.mem.eql(u8, arg, "-f")) {
            i += 1;
            if (i >= args.len) return error.MissingFileValue;
            file = args[i];
        } else if (std.mem.eql(u8, arg, "--compress") or std.mem.eql(u8, arg, "-c")) {
            do_compress = true;
        } else if (std.mem.eql(u8, arg, "--upload") or std.mem.eql(u8, arg, "-u")) {
            do_upload = true;
        } else if (std.mem.eql(u8, arg, "--live")) {
            live = true;
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

    // Generate output file path if not specified
    const output_file = if (file) |f|
        try allocator.dupe(u8, f)
    else
        try generateHeapDumpPath(allocator, target_pid);
    defer allocator.free(output_file);

    std.debug.print("Collecting heap dump for PID {d}...\n", .{target_pid});
    std.debug.print("Output file: {s}\n", .{output_file});

    // Execute jmap to collect heap dump
    const live_flag = if (live) "-live" else "";
    try executeJmap(allocator, target_pid, output_file, live_flag);

    // Track the file to upload (either original or compressed)
    var file_to_upload: []const u8 = output_file;
    var should_free_upload_path = false;

    // Compress if requested
    if (do_compress) {
        std.debug.print("Compressing heap dump...\n", .{});
        const compressed_path = try compress.compressAndDeleteSource(allocator, output_file, null);
        file_to_upload = compressed_path;
        should_free_upload_path = true;
    }
    defer if (should_free_upload_path) allocator.free(file_to_upload);

    // Upload if requested
    if (do_upload) {
        try upload.uploadFileFromEnv(allocator, file_to_upload);
    }

    std.debug.print("Heap dump collection complete\n", .{});
}

fn printHelp() !void {
    std.debug.print(
        \\diagtools heap - Collect Java heap dump
        \\
        \\Usage: diagtools heap [options]
        \\
        \\Options:
        \\  -h, --help          Show this help message
        \\  -p, --pid <PID>     Target process ID
        \\  -n, --name <NAME>   Target process name (finds Java process)
        \\  -f, --file <PATH>   Output file path (auto-generated if not specified)
        \\  -c, --compress      Compress heap dump to ZIP
        \\  -u, --upload        Upload to diagnostic collector service
        \\  --live              Dump only live objects (jmap -live option)
        \\
        \\Examples:
        \\  diagtools heap --pid 12345
        \\  diagtools heap --name myapp --compress --upload
        \\  diagtools heap --name myapp --live
        \\
        \\Environment Variables:
        \\  JAVA_PROCESS_NAME    Default process name if --name not specified
        \\  DIAGCOLLECTOR_URL    URL for upload (required with --upload)
        \\  DIAGCOLLECTOR_TOKEN  Auth token for upload
        \\
    , .{});
}

fn generateHeapDumpPath(allocator: std.mem.Allocator, pid: u32) ![]u8 {
    const timestamp = std.time.timestamp();
    return try std.fmt.allocPrint(
        allocator,
        "heap-dump-{d}-{d}.hprof",
        .{ pid, timestamp },
    );
}

fn executeJmap(allocator: std.mem.Allocator, pid: u32, output_file: []const u8, live_flag: []const u8) !void {
    // Build the actual command with formatted arguments
    const dump_arg = try std.fmt.allocPrint(allocator, "-dump:format=b,file={s}", .{output_file});
    defer allocator.free(dump_arg);

    const pid_str = try std.fmt.allocPrint(allocator, "{d}", .{pid});
    defer allocator.free(pid_str);

    var argv = std.ArrayList([]const u8).empty;
    defer argv.deinit(allocator);

    try argv.append(allocator, "jmap");
    if (live_flag.len > 0) {
        try argv.append(allocator, live_flag);
    }
    try argv.append(allocator, dump_arg);
    try argv.append(allocator, pid_str);

    // Execute jmap command
    var child = std.process.Child.init(argv.items, allocator);
    child.stdout_behavior = .Inherit;
    child.stderr_behavior = .Inherit;

    const term = try child.spawnAndWait();

    switch (term) {
        .Exited => |code| {
            if (code != 0) {
                std.debug.print("jmap exited with code {d}\n", .{code});
                return error.JmapFailed;
            }
        },
        else => {
            std.debug.print("jmap terminated abnormally\n", .{});
            return error.JmapFailed;
        },
    }
}

test "heap command tests" {
    @import("std").testing.refAllDecls(@This());
}
