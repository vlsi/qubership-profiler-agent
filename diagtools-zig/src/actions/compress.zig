const std = @import("std");

/// Compress a file to ZIP format
/// If dest_path is null, appends .zip to source_path
pub fn compressFile(allocator: std.mem.Allocator, source_path: []const u8, dest_path: ?[]const u8) ![]const u8 {
    // Determine output path
    const output_path = if (dest_path) |path|
        try allocator.dupe(u8, path)
    else
        try std.fmt.allocPrint(allocator, "{s}.zip", .{source_path});
    errdefer allocator.free(output_path);

    std.debug.print("Compressing {s} -> {s}\n", .{ source_path, output_path });

    // Open source file
    const source_file = try std.fs.cwd().openFile(source_path, .{});
    defer source_file.close();

    // Get source file size
    const source_stat = try source_file.stat();
    const source_size = source_stat.size;

    // Create output file
    const output_file = try std.fs.cwd().createFile(output_path, .{});
    errdefer {
        output_file.close();
        std.fs.cwd().deleteFile(output_path) catch {};
    }
    defer output_file.close();

    // Use arena allocator for temporary allocations during compression
    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const temp_allocator = arena.allocator();

    // Write ZIP file
    try writeZipFile(temp_allocator, source_file, output_file, source_path, source_size);

    std.debug.print("Compression complete: {s} ({d} bytes)\n", .{ output_path, try output_file.getPos() });

    return output_path;
}

/// Write a ZIP file containing a single file
fn writeZipFile(
    allocator: std.mem.Allocator,
    source_file: std.fs.File,
    output_file: std.fs.File,
    filename: []const u8,
    uncompressed_size: u64,
) !void {
    // Extract just the filename without path
    const basename = std.fs.path.basename(filename);

    // ZIP Local File Header
    const signature: u32 = 0x04034b50; // Local file header signature
    const version: u16 = 20; // Version needed to extract (2.0)
    const flags: u16 = 0; // General purpose bit flag
    const method: u16 = 0; // Compression method (0 = store, no compression)
    const mod_time: u16 = 0; // Last mod file time (MS-DOS format)
    const mod_date: u16 = 0; // Last mod file date (MS-DOS format)
    const filename_len: u16 = @intCast(basename.len);
    const extra_len: u16 = 0;

    // Remember position of local header for directory entry
    const local_header_offset = try output_file.getPos();

    // Helper function to write integers in little endian
    var buf: [4]u8 = undefined;

    // Write local file header
    std.mem.writeInt(u32, &buf, signature, .little);
    _ = try output_file.write(&buf);

    std.mem.writeInt(u16, buf[0..2], version, .little);
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], flags, .little);
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], method, .little);
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], mod_time, .little);
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], mod_date, .little);
    _ = try output_file.write(buf[0..2]);

    const crc_pos = try output_file.getPos();
    std.mem.writeInt(u32, &buf, 0, .little); // CRC32 (placeholder)
    _ = try output_file.write(&buf);

    std.mem.writeInt(u32, &buf, 0, .little); // Compressed size (placeholder)
    _ = try output_file.write(&buf);

    std.mem.writeInt(u32, &buf, 0, .little); // Uncompressed size (placeholder)
    _ = try output_file.write(&buf);

    std.mem.writeInt(u16, buf[0..2], filename_len, .little);
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], extra_len, .little);
    _ = try output_file.write(buf[0..2]);

    _ = try output_file.write(basename);

    // Write file data (store method - no compression)
    const data_start = try output_file.getPos();

    var hasher = std.hash.Crc32.init();

    try source_file.seekTo(0);

    // Read all data
    const all_data = try source_file.readToEndAlloc(allocator, 1024 * 1024 * 1024); // Max 1GB
    defer allocator.free(all_data);

    // Calculate CRC32
    hasher.update(all_data);
    const crc32 = hasher.final();

    // Write data without compression (store method)
    _ = try output_file.write(all_data);
    const compressed_size: u32 = @intCast(all_data.len);

    // Update header with actual CRC and sizes
    try output_file.seekTo(crc_pos);
    std.mem.writeInt(u32, &buf, crc32, .little);
    _ = try output_file.write(&buf);

    std.mem.writeInt(u32, &buf, compressed_size, .little);
    _ = try output_file.write(&buf);

    std.mem.writeInt(u32, &buf, @intCast(uncompressed_size), .little);
    _ = try output_file.write(&buf);

    // Seek to end for central directory
    try output_file.seekTo(data_start + compressed_size);

    // Write Central Directory Header
    const central_dir_offset = try output_file.getPos();

    std.mem.writeInt(u32, &buf, 0x02014b50, .little); // Central directory signature
    _ = try output_file.write(&buf);

    std.mem.writeInt(u16, buf[0..2], 20, .little); // Version made by
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], version, .little); // Version needed to extract
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], flags, .little);
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], method, .little);
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], mod_time, .little);
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], mod_date, .little);
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u32, &buf, crc32, .little);
    _ = try output_file.write(&buf);

    std.mem.writeInt(u32, &buf, compressed_size, .little);
    _ = try output_file.write(&buf);

    std.mem.writeInt(u32, &buf, @intCast(uncompressed_size), .little);
    _ = try output_file.write(&buf);

    std.mem.writeInt(u16, buf[0..2], filename_len, .little);
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], 0, .little); // Extra field length
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], 0, .little); // File comment length
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], 0, .little); // Disk number start
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], 0, .little); // Internal file attributes
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u32, &buf, 0, .little); // External file attributes
    _ = try output_file.write(&buf);

    std.mem.writeInt(u32, &buf, @intCast(local_header_offset), .little); // Relative offset of local header
    _ = try output_file.write(&buf);

    _ = try output_file.write(basename);

    // Write End of Central Directory Record
    const end_of_central_dir_offset = try output_file.getPos();
    const central_dir_size = end_of_central_dir_offset - central_dir_offset;

    std.mem.writeInt(u32, &buf, 0x06054b50, .little); // End of central dir signature
    _ = try output_file.write(&buf);

    std.mem.writeInt(u16, buf[0..2], 0, .little); // Number of this disk
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], 0, .little); // Disk where central directory starts
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], 1, .little); // Number of central directory records on this disk
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u16, buf[0..2], 1, .little); // Total number of central directory records
    _ = try output_file.write(buf[0..2]);

    std.mem.writeInt(u32, &buf, @intCast(central_dir_size), .little); // Size of central directory
    _ = try output_file.write(&buf);

    std.mem.writeInt(u32, &buf, @intCast(central_dir_offset), .little); // Offset of start of central directory
    _ = try output_file.write(&buf);

    std.mem.writeInt(u16, buf[0..2], 0, .little); // ZIP file comment length
    _ = try output_file.write(buf[0..2]);
}

/// Delete the source file after successful compression
pub fn compressAndDeleteSource(allocator: std.mem.Allocator, source_path: []const u8, dest_path: ?[]const u8) ![]const u8 {
    const output_path = try compressFile(allocator, source_path, dest_path);
    errdefer allocator.free(output_path);

    // Delete source file
    try std.fs.cwd().deleteFile(source_path);
    std.debug.print("Deleted source file: {s}\n", .{source_path});

    return output_path;
}

test "compress tests" {
    @import("std").testing.refAllDecls(@This());
}

test "compress small file" {
    const testing = std.testing;
    const allocator = testing.allocator;

    // Create a temporary test file
    const test_data = "Hello, Zig! This is test data for compression.\n" ** 100;
    const test_file = "test_compress_input.txt";
    const zip_file = "test_compress_output.zip";

    // Clean up any existing files
    std.fs.cwd().deleteFile(test_file) catch {};
    std.fs.cwd().deleteFile(zip_file) catch {};

    defer {
        std.fs.cwd().deleteFile(test_file) catch {};
        std.fs.cwd().deleteFile(zip_file) catch {};
    }

    // Write test file
    try std.fs.cwd().writeFile(.{ .sub_path = test_file, .data = test_data });

    // Compress it
    const result = try compressFile(allocator, test_file, zip_file);
    defer allocator.free(result);

    try testing.expectEqualStrings(zip_file, result);

    // Verify ZIP file exists and has content
    const stat = try std.fs.cwd().statFile(zip_file);
    try testing.expect(stat.size > 0);
    try testing.expect(stat.size < test_data.len); // Should be compressed
}
