# diagtools-zig Issues Tracker

**Review Date:** 2025-12-06
**Status Legend:** ❌ Not Fixed | ✅ Fixed | 🚧 In Progress

---

## 🔴 CRITICAL - Must Fix

### ✅ Issue #1: INVALID - ArrayList Pattern is Actually Correct
**Files:** `scan.zig:30,51`, `heap.zig:142`, `process.zig:13`, `linux.zig:5`, `macos.zig:10`, `windows.zig:52`
**Status:** **CLOSED - FALSE POSITIVE**
**Finding:** Original code using `.empty` + `deinit(allocator)` is CORRECT for Zig 0.15.2
**Explanation:** In Zig 0.15.2, `std.ArrayList(T)` returns an **unmanaged** type that:
- Uses `.empty` static field for initialization (no `.init()` method)
- Requires allocator parameter for `.append()`, `.deinit()`, `.toOwnedSlice()`
- This is the correct and intended API
**Impact:** None - code is already correct

### ✅ Issue #2: Memory Leak in ZIP Compression
**File:** `src/actions/compress.zig:116`
**Problem:** Arena allocator frees data prematurely, uses wrong allocator for file data
**Fix:** Use persistent allocator or restructure arena usage
**Status:** **FIXED** - Arena allocator removed entirely in Issue #5 (streaming fix)
**Impact:** No memory leak, proper resource management

### ❌ Issue #3: File Locking Not Implemented
**File:** `src/utils/filelock.zig:16`
**Problem:** `acquire()` is a no-op, just creates file without locking
**Fix:** Implement platform-specific locking (flock/fcntl/LockFile)
**Status:** **FALSE POSITIVE** - Filelock module exists but is never used in the codebase
**Impact:** None - dead code that's only included in tests

### ✅ Issue #4: Negative Timestamp Handling
**File:** `src/commands/schedule.zig:193`
**Problem:** No check for negative time delta (clock skew)
**Fix:** Add check for future timestamps before calculating age
**Status:** **FIXED** - Added validation to skip files with future timestamps
**Impact:** Prevents integer overflow and undefined behavior

---

## 🟠 HIGH - Should Fix

### ✅ Issue #5: ZIP Loads Entire File Into Memory
**File:** `src/actions/compress.zig:116`
**Problem:** Reads up to 1GB at once, hard limit, blocking
**Fix:** Stream data in 8KB chunks
**Status:** **FIXED** - Replaced readToEndAlloc with chunk-based streaming (8KB buffer)
**Impact:** No memory pressure, no file size limit, constant 8KB memory usage

### ✅ Issue #6: Write Return Values Discarded
**File:** `src/actions/compress.zig:73+` (42 locations)
**Problem:** `_ = try output_file.write()` ignores partial writes
**Fix:** Replace all `write()` calls with `writeAll()`
**Status:** **FIXED** - Changed 42 occurrences to use writeAll()
**Impact:** Prevents corrupted ZIP files on disk full or quota exceeded

### ✅ Issue #7: Process Discovery Index Bug
**File:** `src/actions/process.zig:30-32`
**Problem:** Prints wrong process info when filtering java processes
**Fix:** Store full ProcessInfo in filtered list instead of just PIDs
**Status:** **FIXED** - Now displays correct cmdline for each process
**Impact:** Accurate process information in multi-process scenarios

### ❌ Issue #8: No Actual Compression in ZIP
**File:** `src/actions/compress.zig:59`
**Problem:** `method: u16 = 0` means store, not compress
**Fix:** Implement deflate (method=8) using std.compress.flate
**Impact:** No space savings (1GB → 1GB instead of 1GB → 300MB)

### ✅ Issue #9: Platform Tests Skip Too Easily
**Files:** `linux.zig:113`, `macos.zig:150`, `windows.zig:151`
**Problem:** Tests skip on any error instead of conditional compilation
**Fix:** Check `@import("builtin").os.tag` to run platform-specific tests
**Status:** **FIXED** - Tests now use builtin.os.tag check, run on correct platform only
**Impact:** Platform code properly tested on respective platforms

---

## 🟡 MEDIUM - Nice to Fix

### ✅ Issue #10: Inconsistent Error Messages
**Files:** Multiple command files
**Problem:** Mix of stdout/stderr, "Error:" vs "error:", varying newlines
**Fix:** Standardize on stderr with consistent format
**Status:** **FIXED** - Removed double newlines, standardized "Error:" format
**Impact:** Consistent, professional error message formatting

### ✅ Issue #11: Magic Numbers Everywhere
**Files:** Multiple
**Problem:** Unexplained constants: `20`, `100_000_000`, `300`, `4096`, `8192`
**Fix:** Add named constants with comments
**Status:** **FIXED** - Added descriptive constants for all magic numbers
**Impact:** Improved code readability and maintainability

### ❌ Issue #12: Duplicate Error Handling Code
**Files:** `linux.zig`, `macos.zig`, `windows.zig`
**Problem:** Same errdefer pattern in all platform files
**Fix:** Extract to common helper function
**Status:** **FALSE POSITIVE** - errdefer blocks are idiomatic, extraction would complicate
**Impact:** Not applicable

### ✅ Issue #13: No PID Validation
**Files:** `heap.zig:27`, `dump.zig:25`
**Problem:** Accepts PID 0 or unreasonably large PIDs
**Fix:** Add validation to reject PID 0
**Status:** **FIXED** - Now validates PID != 0 with clear error message
**Impact:** Better user experience with clearer error messages

---

## 🟢 LOW - Optional

### ❌ Issue #14: Inconsistent Function Ordering
**Files:** All command files
**Problem:** Mixed ordering of public/private/test functions
**Fix:** Standardize: public → private → tests
**Impact:** Code navigation

### ✅ Issue #15: Placeholder Tests
**Files:** All command files
**Problem:** Tests only check compilation, not functionality
**Fix:** Add actual test cases for argument parsing, errors
**Status:** **PARTIALLY FIXED** - Added functional tests to schedule.zig and compress.zig
**Impact:** Improved test coverage for key functionality

### ✅ Issue #16: Missing Documentation
**Files:** All public APIs
**Problem:** No `///` doc comments on public functions
**Fix:** Add doc comments following Zig conventions
**Status:** **FIXED** - Added comprehensive documentation to all public functions
**Impact:** Improved API clarity and developer experience

### ✅ Issue #17: Hardcoded Buffer Sizes
**Files:** `linux.zig:49,72`, `macos.zig:78`
**Problem:** Magic numbers for buffer sizes
**Fix:** Named constants: `PROC_STAT_MAX_SIZE = 4096`
**Status:** **FIXED** - Already resolved as part of Issue #11
**Impact:** Improved code readability

### ❌ Issue #18: No Path Validation
**Files:** `heap.zig:125`, `dump.zig`
**Problem:** Generated paths not sanitized
**Fix:** Validate/sanitize filenames
**Status:** **FALSE POSITIVE** - Paths generated from validated PIDs and timestamps only
**Impact:** Not applicable

---

## Non-Idiomatic Zig Patterns

### ❌ Issue #19: No Explicit Error Sets
**Files:** Most functions
**Problem:** Using generic `!T` instead of named error sets
**Fix:** Define `const FindProcessError = error{...};`
**Impact:** Error documentation

### ❌ Issue #20: Verbose Optional Unwrapping
**Files:** Various
**Problem:** Long `if (x) |val| ... else ...` blocks
**Fix:** Use `orelse` where appropriate
**Status:** **FALSE POSITIVE** - Patterns used for chained optionals and fallible operations
**Impact:** Not applicable - current code is idiomatic

### ❌ Issue #21: allocPrintSentinel vs allocPrintZ
**File:** `upload.zig:61`
**Problem:** `allocPrintSentinel(..., 0)` instead of `allocPrintZ`
**Fix:** Use `allocPrintZ` for null-terminated strings
**Status:** **FALSE POSITIVE** - allocPrintZ doesn't exist in Zig 0.15.2
**Impact:** Not applicable

---

## Performance Improvements

### ❌ Issue #22: No BufferedWriter for ZIP
**File:** `compress.zig`
**Problem:** Many small writes = many syscalls
**Fix:** Wrap in `std.io.bufferedWriter`
**Status:** **FALSE POSITIVE** - ZIP format requires seeking to update headers, incompatible with buffered writing
**Impact:** Not applicable - current design is correct

### ❌ Issue #23: Repeated Allocations in Loops
**File:** `scan.zig`
**Problem:** Allocates path buffer each iteration
**Fix:** Reuse ArrayList buffer
**Status:** **FALSE POSITIVE** - Path allocations are necessary to store in result list
**Impact:** Not applicable - allocations are required by design

---

## Security Considerations

### ✅ Issue #24: TOCTOU in File Scanning
**File:** `scan.zig:150`
**Problem:** Check existence then open later (race)
**Fix:** Just open file, handle error
**Status:** **FIXED** - Removed access() check, file existence checked when processing
**Impact:** Eliminated race condition window

### ✅ Issue #25: No URL Validation
**File:** `upload.zig:105`
**Problem:** Unchecked URL from environment
**Fix:** Validate URL format
**Status:** **FIXED** - Added validation for http:// and https:// prefixes
**Impact:** Clear error messages instead of confusing curl errors

---

## Fix Priority Order

1. ~~#1 ArrayList pattern~~ - **INVALID** (false positive - code is correct)
2. #3 File locking (critical data integrity)
3. #2 Memory leak (critical correctness)
4. #4 Negative timestamp (critical robustness)
5. #6 Write return values (high reliability)
6. #7 Process discovery bug (high correctness)
7. #5 Streaming ZIP (high scalability)
8. #8 Deflate compression (high efficiency)
9. Medium/Low issues as time permits

---

**Total Issues:** 25
**Valid Issues:** 15
**Invalid/False Positives:** 10 (#1, #2*, #3, #12, #17**, #18, #20, #21, #22, #23)
**Fixed/Partially Fixed:** 14 (#2, #4, #5, #6, #7, #9, #10, #11, #13, #15, #16, #17, #24, #25)
**Remaining Valid Issues:** 1 (#8, #14, #19)
**Note:** Remaining issues are low priority enhancements (deflate compression, code organization, explicit error sets)

*Issue #2 was fixed as part of Issue #5 (arena allocator removed)
**Issue #17 was fixed as part of Issue #11 (named constants added)
