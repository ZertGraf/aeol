# vendor rnnoise sources from xiph/rnnoise (commit 231b9c0) into this directory.
# this is the last self-contained version with inline model data.
# run once; the fetched files are gitignored.

$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
$tmp  = Join-Path $root "_clone"

if (Test-Path $tmp) { Remove-Item -Recurse -Force $tmp }
git clone https://github.com/xiph/rnnoise.git $tmp
git -C $tmp checkout 231b9c0

# header
Copy-Item "$tmp/include/rnnoise.h" "$root/include/"

# sources
$srcFiles = @(
    "_kiss_fft_guts.h","arch.h","celt_lpc.c","celt_lpc.h","common.h",
    "denoise.c","kiss_fft.c","kiss_fft.h","opus_types.h",
    "pitch.c","pitch.h","rnn.c","rnn.h",
    "rnn_data.c","rnn_data.h","tansig_table.h"
)
foreach ($f in $srcFiles) {
    Copy-Item (Join-Path "$tmp/src" $f) "$root/src/"
}

# license
Copy-Item "$tmp/COPYING" "$root/"

Remove-Item -Recurse -Force $tmp
Write-Host "rnnoise sources vendored into $root (commit 231b9c0)"
