const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    // Define executable
    const exe = b.addExecutable(.{
        .name = "diagtools",
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = target,
            .optimize = optimize,
        }),
    });

    // Link libcurl for cross-platform HTTP upload support
    exe.linkSystemLibrary("curl");
    exe.linkLibC();

    // Install artifact
    b.installArtifact(exe);

    // Create run step
    const run_cmd = b.addRunArtifact(exe);
    run_cmd.step.dependOn(b.getInstallStep());

    // Forward arguments to executable
    if (b.args) |args| {
        run_cmd.addArgs(args);
    }

    const run_step = b.step("run", "Run the diagtools application");
    run_step.dependOn(&run_cmd.step);

    // Test setup
    const unit_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = target,
            .optimize = optimize,
        }),
    });

    // Link libraries for tests as well
    unit_tests.linkSystemLibrary("curl");
    unit_tests.linkLibC();

    const run_unit_tests = b.addRunArtifact(unit_tests);
    const test_step = b.step("test", "Run unit tests");
    test_step.dependOn(&run_unit_tests.step);

    // Add individual module tests
    const test_modules = [_][]const u8{
        "src/actions/process.zig",
        "src/actions/compress.zig",
        "src/utils/filelock.zig",
    };

    for (test_modules) |module| {
        const module_test = b.addTest(.{
            .root_module = b.createModule(.{
                .root_source_file = b.path(module),
                .target = target,
                .optimize = optimize,
            }),
        });
        module_test.linkSystemLibrary("curl");
        module_test.linkLibC();
        run_unit_tests.step.dependOn(&b.addRunArtifact(module_test).step);
    }

    // Cross-compilation convenience targets
    const targets = [_]std.Target.Query{
        .{ .cpu_arch = .aarch64, .os_tag = .macos },
        .{ .cpu_arch = .x86_64, .os_tag = .windows },
        .{ .cpu_arch = .x86_64, .os_tag = .linux },
        .{ .cpu_arch = .aarch64, .os_tag = .linux },
    };

    const build_all = b.step("build-all", "Build for all target platforms");

    for (targets) |t| {
        const resolved_target = b.resolveTargetQuery(t);
        const target_exe = b.addExecutable(.{
            .name = "diagtools",
            .root_module = b.createModule(.{
                .root_source_file = b.path("src/main.zig"),
                .target = resolved_target,
                .optimize = .ReleaseSafe,
            }),
        });
        target_exe.linkSystemLibrary("curl");
        target_exe.linkLibC();

        const target_name = b.fmt("{s}-{s}", .{
            @tagName(t.cpu_arch.?),
            @tagName(t.os_tag.?),
        });
        const install_step = b.addInstallArtifact(target_exe, .{
            .dest_dir = .{
                .override = .{
                    .custom = b.fmt("bin/{s}", .{target_name}),
                },
            },
        });
        build_all.dependOn(&install_step.step);
    }
}
