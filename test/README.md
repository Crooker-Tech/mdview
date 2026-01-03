# mdview Test Suite & Benchmark Baselines

This document describes the test suite and captures performance baselines for regression detection.

**System:** AMD Ryzen 9 7845HX, Windows, Go 1.24+
**Date:** 2026-01-03
**Commit:** After --preload feature implementation

---

## Test Suite Overview

**Total: 37 tests** across 3 packages

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific test
go test -v -run TestPreloadOptimistic ./converter
```

---

## Test Inventory

### converter/ (29 tests)

#### Path Rewriting Tests
| Test | Description |
|------|-------------|
| `TestMarkdownImagePathRewriting` | Verifies `![img](path)` rewrites: relative→file://, http unchanged, data URI unchanged, missing file fallback |
| `TestMarkdownLinkPathRewriting` | Verifies `[link](path)` rewrites: relative→file://, http/https/mailto/tel/anchor unchanged |
| `TestRawHTMLImageProcessing` | Handles `<img src="">` in raw HTML: rewrite, embed, http unchanged, file:// URL handling |
| `TestRawHTMLLinkProcessing` | Handles `<a href="">` in raw HTML: rewrite, http unchanged, anchor unchanged |
| `TestFileURLHandling` | Existing file:// URLs: embed when self-contained, pass through otherwise |

#### MIME Type Detection Tests
| Test | Description |
|------|-------------|
| `TestMIMETypeDetection` | Image MIME types: png, jpg, jpeg, gif, svg, webp, ico, bmp, avif, unknown |
| `TestCSSAssetMIMETypeDetection` | CSS asset MIME types: woff2, woff, ttf, otf, eot, png, jpg, svg, cur, unknown |

#### CSS Processing Tests
| Test | Description |
|------|-------------|
| `TestInlineStyleCSSURLProcessing` | Processes `url()` references in inline styles |

#### Edge Case Tests
| Test | Description |
|------|-------------|
| `TestNoBaseDirNoRewriting` | No rewriting when baseDir is not set |
| `TestMixedContent` | Document with images, links, and raw HTML mixed together |
| `TestHTMLBlockProcessing` | Multi-line HTML blocks are processed correctly |

#### Markdown Rendering Tests
| Test | Description |
|------|-------------|
| `TestBasicMarkdownRendering` | Core elements: headings, paragraphs, bold/italic, lists, code, blockquotes, hr |
| `TestGFMExtensions` | GitHub Flavored Markdown: tables, task lists, strikethrough, autolinks |
| `TestAutoHeadingIDs` | Headings get automatic `id` attributes |
| `TestTypographer` | Smart quotes (`"` → `&ldquo;`), ellipsis (`...` → `&hellip;`) |
| `TestUnsafeHTMLPassthrough` | Raw HTML passes through with `html.WithUnsafe()` |

#### Output Structure Tests
| Test | Description |
|------|-------------|
| `TestHTMLStructure` | Output has DOCTYPE, html, head, body, article.markdown-body |
| `TestTemplateIntegration` | CSS and JS from template are included in output |

#### Robustness Tests
| Test | Description |
|------|-------------|
| `TestEmptyInput` | Empty markdown produces valid HTML structure |
| `TestSpecialCharacters` | Unicode, emoji, HTML entities handled correctly |
| `TestVeryLongLines` | Lines up to 10KB render without issues |
| `TestDeepNesting` | 50 levels of nested blockquotes render correctly |

#### Memory & Performance Tests
| Test | Description |
|------|-------------|
| `TestMemoryUsageStreaming` | Verifies peak heap (~28x input) and retained memory (~3x output) |
| `TestBufferPoolReuse` | Buffer pool works correctly across 100 conversions |

#### Image Preload Tests
| Test | Description |
|------|-------------|
| `TestImageCacheBasicOperations` | Cache Get/Set operations work correctly |
| `TestImageCachePreloadDirectory` | Async directory preload populates cache |
| `TestPreloadPessimisticCase` | 10 images in dir, only 2 used - still works correctly |
| `TestPreloadOptimisticCase` | 10 images in dir, 8 used - all embedded correctly |
| `TestPreloadVsNoPreloadCorrectness` | Preload produces identical output to non-preload |
| `TestPreloadNonBlockingBehavior` | First image doesn't block waiting for preload |
| `TestPreloadCacheReuse` | Same image referenced 3x only read once |
| `TestPreloadConcurrentSafety` | 10 concurrent conversions with no races |

---

### output/ (4 tests)

| Test | Description |
|------|-------------|
| `TestGetOutputPathWithSpecifiedPath` | Returns specified path unchanged |
| `TestGetOutputPathCreatesParentDirs` | Creates nested directories if needed |
| `TestGetOutputPathGeneratesRandomFile` | Empty path generates random filename in temp dir |
| `TestGetOutputPathRandomFilenameLength` | Random filename is 16 hex chars + .html |

---

### templates/ (4 tests)

| Test | Description |
|------|-------------|
| `TestGetDefaultTemplate` | Default template has CSS (markdown-body) and JS (hljs) |
| `TestGetNonexistentTemplate` | Returns "not found" error for missing template |
| `TestListTemplates` | Lists available templates, includes "default" |
| `TestTemplateCSSDarkModeDefault` | CSS contains `prefers-color-scheme` media query |

---

## How to Run Benchmarks

```bash
# Run all benchmarks (3x for stability)
go test -bench=. -benchmem -count=3 ./converter

# Run specific benchmark
go test -bench=BenchmarkConvertSmall -benchmem -count=5 ./converter

# Compare against saved baseline
go test -bench=. -benchmem ./converter | tee new_results.txt
# Then compare manually or use benchstat
```

## Regression Thresholds

Consider investigating if:
- Time increases by **>15%** from baseline median
- Memory increases by **>20%** from baseline
- Allocations increase at all (should be stable)

---

## Baseline Benchmarks

### 1. Basic Markdown Conversion

| Benchmark | Median Time | Range | Memory | Allocs | Throughput |
|-----------|-------------|-------|--------|--------|------------|
| **ConvertSmall** (~100B) | 131 µs | 107-136 µs | 482 KB | 300 | ~7,600/sec |
| **ConvertMedium** (~5KB) | 1.06 ms | 1.04-1.17 ms | 1.51 MB | 5,197 | ~940/sec |
| **ConvertLarge** (~50KB) | 3.98 ms | 3.91-4.05 ms | 4.72 MB | 15,315 | ~250/sec |

**Expected scaling:** Linear (~10x input → ~10x time)

---

### 2. Image Handling

| Benchmark | Median Time | Range | Memory | Allocs |
|-----------|-------------|-------|--------|--------|
| **Rewrite** (file:// URLs) | 181 µs | 170-193 µs | 531 KB | 494 |
| **Embed** (base64, 1 image) | 4.88 ms | 4.66-5.09 ms | 575 KB | 581 |

**Note:** Embed is ~27x slower due to disk I/O + base64 encoding.

---

### 3. Self-Contained Mode (15 of 20 images used)

| Benchmark | Median Time | Range | Memory | Allocs | vs Baseline |
|-----------|-------------|-------|--------|--------|-------------|
| **NoPreload** | 7.21 ms | 7.19-7.35 ms | 642 KB | 549 | baseline |
| **WithPreload** | 5.88 ms | 5.75-6.07 ms | 765 KB | 830 | **-18% time** |

---

### 4. Preload Scenarios (20 images in directory)

| Benchmark | Images Used | Median Time | Range | Memory | Allocs |
|-----------|-------------|-------------|-------|--------|--------|
| **Pessimistic** | 2 of 20 | 5.42 ms | 5.30-5.62 ms | 654 KB | 640 |
| **Optimistic** | 18 of 20 | 5.93 ms | 5.77-6.13 ms | 765 KB | 862 |

**Key insight:** Preload helps even in pessimistic case due to non-blocking design.

---

### 5. Image Size Impact (8 of 10 images used)

| Image Size | Mode | Median Time | Range | Memory | Allocs |
|------------|------|-------------|-------|--------|--------|
| **Small** (~1KB) | NoPreload | 4.02 ms | 3.94-4.13 ms | 621 KB | 429 |
| **Small** (~1KB) | WithPreload | 3.50 ms | 3.49-3.51 ms | 714 KB | 578 |
| **Large** (~100KB) | NoPreload | 4.12 ms | 4.06-4.19 ms | 804 KB | 432 |
| **Large** (~100KB) | WithPreload | 3.44 ms | 3.38-3.48 ms | 904 KB | 578 |

**Improvement with preload:**
- Small images: **-13%**
- Large images: **-16%**

---

### 6. Size Hint Impact

| Benchmark | Median Time | Memory | Allocs |
|-----------|-------------|--------|--------|
| **Without hint** | 472 µs | 848 KB | 1,794 |
| **With hint** | 462 µs | 848 KB | 1,794 |

**Conclusion:** Negligible difference (~2%). Buffer pool handles this well.

---

### 7. Memory Allocation

| Benchmark | Median Time | Memory | Allocs |
|-----------|-------------|--------|--------|
| **MemoryAllocation** | 311 µs | 653 KB | 1,038 |

---

## Memory Profiling Baselines

For 1.35 MB markdown input:

| Metric | Value | Ratio to Input |
|--------|-------|----------------|
| Input size | 1.35 MB | 1x |
| Output size | 1.75 MB | 1.3x |
| Peak heap | 38.6 MB | 28.5x |
| Retained after GC | 3.77 MB | 2.8x |
| Total allocated | 45.0 MB | 33x |

**Note:** Peak heap is high due to goldmark AST nodes (not a leak).
**Key metric:** Retained after GC should stay ~2-3x output size.

---

## Variance Notes

Benchmark variance on this system is typically **10-25%** due to:
- CPU boost states (Ryzen dynamic boost)
- Thermal throttling
- Background processes
- Disk cache state (for image tests)

When comparing:
1. Run benchmarks **3-5 times**
2. Use **median** not mean
3. Ignore differences **<15%**
4. Run on **idle system** for best results

---

## Historical Baselines

### v1.0.0 (2026-01-03) - Initial + Preload

```
BenchmarkConvertSmall-24                     131 µs    482 KB    300 allocs
BenchmarkConvertMedium-24                   1.06 ms   1.51 MB   5197 allocs
BenchmarkConvertLarge-24                    3.98 ms   4.72 MB  15315 allocs
BenchmarkConvertWithImages/rewrite-24        181 µs    531 KB    494 allocs
BenchmarkConvertWithImages/embed-24         4.88 ms    575 KB    581 allocs
BenchmarkSelfContainedNoPreload-24          7.21 ms    642 KB    549 allocs
BenchmarkSelfContainedWithPreload-24        5.88 ms    765 KB    830 allocs
BenchmarkPreloadPessimistic-24              5.42 ms    654 KB    640 allocs
BenchmarkPreloadOptimistic-24               5.93 ms    765 KB    862 allocs
BenchmarkSmallImagesNoPreload-24            4.02 ms    621 KB    429 allocs
BenchmarkSmallImagesWithPreload-24          3.50 ms    714 KB    578 allocs
BenchmarkLargeImagesNoPreload-24            4.12 ms    804 KB    432 allocs
BenchmarkLargeImagesWithPreload-24          3.44 ms    904 KB    578 allocs
```

---

## Adding New Baselines

When adding features, update this file:

1. Run full benchmark suite: `go test -bench=. -benchmem -count=3 ./converter`
2. Calculate medians for new benchmarks
3. Add to appropriate section above
4. Add entry to Historical Baselines with date/commit info
