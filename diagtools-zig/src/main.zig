const std = @import("std");
// TODO: Re-enable when zig-clap is compatible with Zig 0.15.2
// const clap = @import("clap");

const heap_cmd = @import("commands/heap.zig");
const dump_cmd = @import("commands/dump.zig");
const scan_cmd = @import("commands/scan.zig");
const schedule_cmd = @import("commands/schedule.zig");
const config_cmd = @import("commands/config.zig");

const version = "1.0.0";

pub fn main() !void {
    // Set up GeneralPurposeAllocator with leak detection
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer {
        const deinit_status = gpa.deinit();
        if (deinit_status == .leak) {
            std.debug.print("Warning: Memory leak detected!\n", .{});
        }
    }
    const allocator = gpa.allocator();

    // Simple argument parsing without zig-clap for now
    const args = try std.process.argsAlloc(allocator);
    defer std.process.argsFree(allocator, args);

    if (args.len < 2) {
        try printHelp();
        return;
    }

    // Check for global flags
    for (args[1..]) |arg| {
        if (std.mem.eql(u8, arg, "--help") or std.mem.eql(u8, arg, "-h")) {
            try printHelp();
            return;
        }
        if (std.mem.eql(u8, arg, "--version") or std.mem.eql(u8, arg, "-v")) {
            std.debug.print("diagtools version {s}\n", .{version});
            return;
        }
    }

    const subcommand = args[1];

    // Dispatch to appropriate command handler
    if (std.mem.eql(u8, subcommand, "heap")) {
        try heap_cmd.run(allocator, args[2..]);
    } else if (std.mem.eql(u8, subcommand, "dump")) {
        try dump_cmd.run(allocator, args[2..]);
    } else if (std.mem.eql(u8, subcommand, "scan")) {
        try scan_cmd.run(allocator, args[2..]);
    } else if (std.mem.eql(u8, subcommand, "schedule")) {
        try schedule_cmd.run(allocator, args[2..]);
    } else if (std.mem.eql(u8, subcommand, "config")) {
        try config_cmd.run(allocator, args[2..]);
    } else {
        std.debug.print("Unknown command: {s}\n\n", .{subcommand});
        try printHelp();
        return error.UnknownCommand;
    }
}

fn printHelp() !void {
    std.debug.print(
        \\diagtools - Java application diagnostics collector
        \\
        \\Usage: diagtools [options] <command> [command-options]
        \\
        \\Commands:
        \\  heap       Collect heap dump from Java process
        \\  dump       Collect thread dumps and CPU usage
        \\  scan       Scan and upload diagnostic files
        \\  schedule   Run diagnostic collection on a schedule
        \\  config     Export configuration from Consul/Config Server
        \\
        \\Options:
        \\  -h, --help     Show this help message
        \\  -v, --version  Show version information
        \\
        \\Environment Variables:
        \\  DIAGCOLLECTOR_URL    URL of diagnostic collection service
        \\  DIAGCOLLECTOR_TOKEN  Authentication token
        \\  JAVA_PROCESS_NAME    Name of Java process to monitor
        \\  LOG_FILE             Path to log file (default: diagtools.log)
        \\  LOG_MAX_SIZE         Maximum log file size in MB (default: 100)
        \\  LOG_MAX_AGE          Maximum log file age in days (default: 7)
        \\
        \\For command-specific help, use: diagtools <command> --help
        \\
        \\Note: This build uses simplified argument parsing. For full clap support,
        \\      update to zig-clap compatible with Zig 0.15.2
        \\
    , .{});
}

// Test entry point
test "main tests" {
    @import("std").testing.refAllDecls(@This());
}
