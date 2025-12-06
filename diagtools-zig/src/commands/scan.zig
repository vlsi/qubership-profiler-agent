const std = @import("std");
const compress = @import("../actions/compress.zig");
const upload = @import("../actions/upload.zig");

pub fn run(allocator: std.mem.Allocator, args: []const []const u8) !void {
    var show_help = false;

    // Check for help flag
    for (args) |arg| {
        if (std.mem.eql(u8, arg, "--help") or std.mem.eql(u8, arg, "-h")) {
            show_help = true;
            break;
        }
    }

    if (show_help) {
        try printHelp();
        return;
    }

    if (args.len == 0) {
        std.debug.print("Error: No file patterns specified\n\n", .{});
        try printHelp();
        return error.MissingPatterns;
    }

    std.debug.print("Scanning for diagnostic files...\n", .{});

    // Collect all matching files
    var files_to_process = std.ArrayList([]const u8).empty;
    defer {
        for (files_to_process.items) |file| {
            allocator.free(file);
        }
        files_to_process.deinit(allocator);
    }

    // Scan for files matching patterns
    for (args) |pattern| {
        try scanPattern(allocator, pattern, &files_to_process);
    }

    if (files_to_process.items.len == 0) {
        std.debug.print("No files found matching the specified patterns\n", .{});
        return;
    }

    std.debug.print("Found {d} file(s) to process\n", .{files_to_process.items.len});

    // Track files to upload
    var files_to_upload = std.ArrayList([]const u8).empty;
    defer {
        for (files_to_upload.items) |file| {
            allocator.free(file);
        }
        files_to_upload.deinit(allocator);
    }

    // Process each file
    for (files_to_process.items) |file_path| {
        std.debug.print("Processing: {s}\n", .{file_path});

        // Check if file needs compression (.hprof files)
        if (std.mem.endsWith(u8, file_path, ".hprof")) {
            std.debug.print("  Compressing .hprof file...\n", .{});

            // Compress and delete the .hprof file
            const compressed_path = try compress.compressAndDeleteSource(allocator, file_path, null);
            try files_to_upload.append(allocator, compressed_path);

            std.debug.print("  Compressed to: {s}\n", .{compressed_path});
        } else {
            // File is already in uploadable format (e.g., .hprof.zip, .td.txt, .top.txt)
            const file_copy = try allocator.dupe(u8, file_path);
            try files_to_upload.append(allocator, file_copy);
        }
    }

    // Upload files if enabled
    const should_upload = std.posix.getenv("DIAGCOLLECTOR_URL") != null;

    if (should_upload and files_to_upload.items.len > 0) {
        std.debug.print("\nUploading {d} file(s)...\n", .{files_to_upload.items.len});

        for (files_to_upload.items, 0..) |file_path, i| {
            std.debug.print("  [{d}/{d}] Uploading: {s}\n", .{ i + 1, files_to_upload.items.len, file_path });

            upload.uploadFileFromEnv(allocator, file_path) catch |err| {
                std.debug.print("  Warning: Upload failed for {s}: {}\n", .{ file_path, err });
                continue;
            };

            // Delete file after successful upload
            std.fs.cwd().deleteFile(file_path) catch |err| {
                std.debug.print("  Warning: Failed to delete {s}: {}\n", .{ file_path, err });
            };
        }

        std.debug.print("Upload complete\n", .{});
    } else if (!should_upload) {
        std.debug.print("\nSkipping upload (DIAGCOLLECTOR_URL not set)\n", .{});
        std.debug.print("Files ready for upload:\n", .{});
        for (files_to_upload.items) |file_path| {
            std.debug.print("  {s}\n", .{file_path});
        }
    }

    std.debug.print("Scan complete\n", .{});
}

fn printHelp() !void {
    std.debug.print(
        \\diagtools scan - Scan and upload diagnostic files
        \\
        \\Usage: diagtools scan [options] <pattern...>
        \\
        \\Arguments:
        \\  pattern...              File patterns to scan (e.g., "*.hprof*" "./core*" "*.td.txt")
        \\                          Supports glob patterns and multiple patterns
        \\
        \\Options:
        \\  -h, --help              Show this help message
        \\
        \\Description:
        \\  Scans for diagnostic files matching the specified patterns.
        \\  - .hprof files are automatically compressed to .hprof.zip
        \\  - Other files (.hprof.zip, .td.txt, .top.txt, etc.) are uploaded as-is
        \\  - Files are uploaded if DIAGCOLLECTOR_URL is set
        \\  - Successfully uploaded files are deleted
        \\
        \\Examples:
        \\  diagtools scan "*.hprof*"
        \\  diagtools scan "/var/log/dumps/*.hprof*" "./core*" "./hs_err*"
        \\  diagtools scan "*.td.txt" "*.top.txt"
        \\
        \\Environment Variables:
        \\  DIAGCOLLECTOR_URL       URL for upload (required for auto-upload)
        \\  DIAGCOLLECTOR_TOKEN     Auth token for upload
        \\
    , .{});
}

fn scanPattern(allocator: std.mem.Allocator, pattern: []const u8, files: *std.ArrayList([]const u8)) !void {
    // Check if pattern contains wildcards
    const has_wildcard = std.mem.indexOf(u8, pattern, "*") != null or
        std.mem.indexOf(u8, pattern, "?") != null;

    if (!has_wildcard) {
        // No wildcard - check if file exists
        std.fs.cwd().access(pattern, .{}) catch |err| {
            std.debug.print("Warning: File not found: {s} ({any})\n", .{ pattern, err });
            return;
        };

        const file_path = try allocator.dupe(u8, pattern);
        try files.append(allocator, file_path);
        return;
    }

    // Pattern has wildcards - need to expand it
    // For simplicity, we'll use a basic glob implementation
    // Split pattern into directory and filename parts
    const last_sep = std.mem.lastIndexOfScalar(u8, pattern, '/');

    if (last_sep) |sep_idx| {
        // Pattern has directory component
        const dir_path = pattern[0..sep_idx];
        const file_pattern = pattern[sep_idx + 1 ..];

        var dir = std.fs.cwd().openDir(dir_path, .{ .iterate = true }) catch |err| {
            std.debug.print("Warning: Cannot open directory {s}: {any}\n", .{ dir_path, err });
            return;
        };
        defer dir.close();

        var iter = dir.iterate();
        while (try iter.next()) |entry| {
            if (entry.kind != .file) continue;

            // Simple pattern matching
            if (matchPattern(entry.name, file_pattern)) {
                const full_path = try std.fmt.allocPrint(allocator, "{s}/{s}", .{ dir_path, entry.name });
                try files.append(allocator, full_path);
            }
        }
    } else {
        // Pattern is just filename in current directory
        var dir = std.fs.cwd().openDir(".", .{ .iterate = true }) catch |err| {
            std.debug.print("Warning: Cannot open current directory: {any}\n", .{err});
            return;
        };
        defer dir.close();

        var iter = dir.iterate();
        while (try iter.next()) |entry| {
            if (entry.kind != .file) continue;

            if (matchPattern(entry.name, pattern)) {
                const file_path = try allocator.dupe(u8, entry.name);
                try files.append(allocator, file_path);
            }
        }
    }
}

fn matchPattern(filename: []const u8, pattern: []const u8) bool {
    // Simple glob pattern matching
    // Supports * (matches any sequence) and ? (matches single char)

    var fi: usize = 0;
    var pi: usize = 0;
    var star_idx: ?usize = null;
    var match_idx: usize = 0;

    while (fi < filename.len) {
        if (pi < pattern.len) {
            if (pattern[pi] == '*') {
                star_idx = pi;
                match_idx = fi;
                pi += 1;
                continue;
            }

            if (pattern[pi] == '?' or pattern[pi] == filename[fi]) {
                pi += 1;
                fi += 1;
                continue;
            }
        }

        if (star_idx) |star| {
            pi = star + 1;
            match_idx += 1;
            fi = match_idx;
            continue;
        }

        return false;
    }

    // Skip remaining asterisks in pattern
    while (pi < pattern.len and pattern[pi] == '*') {
        pi += 1;
    }

    return pi == pattern.len;
}

test "scan command tests" {
    @import("std").testing.refAllDecls(@This());
}

test "pattern matching" {
    const testing = std.testing;

    try testing.expect(matchPattern("test.hprof", "*.hprof"));
    try testing.expect(matchPattern("test.hprof.zip", "*.hprof*"));
    try testing.expect(matchPattern("heap-dump.hprof", "*dump*.hprof"));
    try testing.expect(matchPattern("test.txt", "test.txt"));
    try testing.expect(matchPattern("file123.log", "file???.log"));

    try testing.expect(!matchPattern("test.txt", "*.hprof"));
    try testing.expect(!matchPattern("test.hprof", "*.zip"));
    try testing.expect(!matchPattern("file1.log", "file???.log"));
}
