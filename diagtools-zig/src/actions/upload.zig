const std = @import("std");

// libcurl C bindings
const c = @cImport({
    @cInclude("curl/curl.h");
});

// HTTP upload constants
const UPLOAD_TIMEOUT_SECONDS = 300; // 5 minutes timeout for uploads
const HTTP_SUCCESS_MIN = 200; // Minimum HTTP success status code
const HTTP_SUCCESS_MAX = 299; // Maximum HTTP success status code

/// Upload a file to a URL using HTTP multipart POST
pub fn uploadFile(
    allocator: std.mem.Allocator,
    file_path: []const u8,
    url: []const u8,
    token: ?[]const u8,
) !void {
    std.debug.print("Uploading {s} to {s}\n", .{ file_path, url });

    // Initialize curl
    const curl = c.curl_easy_init() orelse return error.CurlInitFailed;
    defer c.curl_easy_cleanup(curl);

    // Prepare URL (null-terminated for C)
    const url_z = try allocator.dupeZ(u8, url);
    defer allocator.free(url_z);

    const file_path_z = try allocator.dupeZ(u8, file_path);
    defer allocator.free(file_path_z);

    // Set URL
    _ = c.curl_easy_setopt(curl, c.CURLOPT_URL, url_z.ptr);

    // Create multipart form
    var form: ?*c.curl_httppost = null;
    var last: ?*c.curl_httppost = null;

    // Extract just the filename
    const filename = std.fs.path.basename(file_path);
    const filename_z = try allocator.dupeZ(u8, filename);
    defer allocator.free(filename_z);

    // Add file to form
    _ = c.curl_formadd(
        &form,
        &last,
        c.CURLFORM_COPYNAME,
        "file",
        c.CURLFORM_FILE,
        file_path_z.ptr,
        c.CURLFORM_FILENAME,
        filename_z.ptr,
        c.CURLFORM_END,
    );

    // Set the form
    _ = c.curl_easy_setopt(curl, c.CURLOPT_HTTPPOST, form);

    // Add authorization header if token provided
    var headers: ?*c.curl_slist = null;
    defer if (headers) |h| c.curl_slist_free_all(h);

    if (token) |t| {
        const auth_header = try std.fmt.allocPrintSentinel(allocator, "Authorization: Bearer {s}", .{t}, 0);
        defer allocator.free(auth_header);

        headers = c.curl_slist_append(headers, auth_header.ptr);
        _ = c.curl_easy_setopt(curl, c.CURLOPT_HTTPHEADER, headers);
    }

    // Follow redirects
    _ = c.curl_easy_setopt(curl, c.CURLOPT_FOLLOWLOCATION, @as(c_long, 1));

    // Set timeout
    _ = c.curl_easy_setopt(curl, c.CURLOPT_TIMEOUT, @as(c_long, UPLOAD_TIMEOUT_SECONDS));

    // Enable verbose output in debug builds
    if (@import("builtin").mode == .Debug) {
        _ = c.curl_easy_setopt(curl, c.CURLOPT_VERBOSE, @as(c_long, 1));
    }

    // Perform the request
    const res = c.curl_easy_perform(curl);

    // Free the form
    c.curl_formfree(form);

    if (res != c.CURLE_OK) {
        const error_str = c.curl_easy_strerror(res);
        std.debug.print("Upload failed: {s}\n", .{error_str});
        return error.UploadFailed;
    }

    // Check HTTP response code
    var response_code: c_long = 0;
    _ = c.curl_easy_getinfo(curl, c.CURLINFO_RESPONSE_CODE, &response_code);

    std.debug.print("Upload completed with HTTP status: {d}\n", .{response_code});

    if (response_code < HTTP_SUCCESS_MIN or response_code > HTTP_SUCCESS_MAX) {
        std.debug.print("Server returned error status code\n", .{});
        return error.ServerError;
    }
}

/// Upload file using environment variables for configuration
pub fn uploadFileFromEnv(allocator: std.mem.Allocator, file_path: []const u8) !void {
    const url = std.posix.getenv("DIAGCOLLECTOR_URL") orelse {
        std.debug.print("Error: DIAGCOLLECTOR_URL environment variable not set\n", .{});
        return error.MissingUrl;
    };

    const token = std.posix.getenv("DIAGCOLLECTOR_TOKEN");

    try uploadFile(allocator, file_path, url, token);
}

test "upload tests" {
    @import("std").testing.refAllDecls(@This());
}
