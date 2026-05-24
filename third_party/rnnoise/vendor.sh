#!/bin/sh
# vendor rnnoise sources from xiph/rnnoise (commit 231b9c0) into this directory.
# this is the last self-contained version with inline model data.
# run once; the fetched files are gitignored.
set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
TMP="$ROOT/_clone"

rm -rf "$TMP"
git clone https://github.com/xiph/rnnoise.git "$TMP"
git -C "$TMP" checkout 231b9c0

cp "$TMP/include/rnnoise.h" "$ROOT/include/"

for f in _kiss_fft_guts.h arch.h celt_lpc.c celt_lpc.h common.h \
         denoise.c kiss_fft.c kiss_fft.h opus_types.h \
         pitch.c pitch.h rnn.c rnn.h \
         rnn_data.c rnn_data.h tansig_table.h; do
    cp "$TMP/src/$f" "$ROOT/src/"
done

cp "$TMP/COPYING" "$ROOT/"

rm -rf "$TMP"
echo "rnnoise sources vendored into $ROOT (commit 231b9c0)"
