const std = @import("std");

/// File lock handle
pub const FileLock = struct {
    file: std.fs.File,

    /// Acquire an exclusive file lock
    pub fn acquire(path: []const u8) !FileLock {
        const file = try std.fs.cwd().createFile(path, .{
            .read = true,
            .truncate = false,
        });
        errdefer file.close();

        // Platform-specific locking will be implemented here
        _ = file;

        return FileLock{ .file = file };
    }

    /// Release the file lock
    pub fn release(self: *FileLock) void {
        self.file.close();
    }
};

test "filelock tests" {
    @import("std").testing.refAllDecls(@This());
}
