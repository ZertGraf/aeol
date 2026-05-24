/*
 * sonora C API
 *
 * independent audio processing stages from the sonora Go library,
 * exposed as a C shared library.
 *
 * all stages operate on FloatS16 data: float samples in [-32768, 32767].
 * each handle is single-channel. create one handle per channel.
 * handles are NOT thread-safe — protect concurrent access externally.
 *
 * build:
 *   go build -buildmode=c-shared -o sonora.dll  ./capi/   (windows)
 *   go build -buildmode=c-shared -o libsonora.so ./capi/   (linux)
 *   go build -buildmode=c-shared -o libsonora.dylib ./capi/ (macos)
 */

#ifndef SONORA_H
#define SONORA_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef uintptr_t sonora_handle;

/* ---- noise suppressor -------------------------------------------------- */
/* operates on 160-sample frames (10ms at 16kHz).                           */
/* level: 0=low, 1=moderate, 2=high, 3=very_high                           */

sonora_handle sonora_ns_create(int level);
void          sonora_ns_process(sonora_handle h, float *data, int n);
void          sonora_ns_process_upper_band(sonora_handle h, float *data, int n, int band);
void          sonora_ns_reset(sonora_handle h);
void          sonora_ns_destroy(sonora_handle h);

/* ---- echo canceller (aec3) --------------------------------------------- */
/* processes 64-sample blocks internally; pass any multiple of 64.          */
/* call process_render BEFORE process_capture for each frame pair.          */

sonora_handle sonora_aec3_create(uint32_t sample_rate);
void          sonora_aec3_process_render(sonora_handle h, float *data, int n);
void          sonora_aec3_process_capture(sonora_handle h, float *data, int n);
float         sonora_aec3_erle(sonora_handle h);
int           sonora_aec3_delay(sonora_handle h);
void          sonora_aec3_reset(sonora_handle h);
void          sonora_aec3_destroy(sonora_handle h);

/* ---- automatic gain control (agc2) ------------------------------------- */
/* full-band, any frame size. default config: adaptive+fixed gain.          */

sonora_handle sonora_agc2_create(void);
sonora_handle sonora_agc2_create_ex(float fixed_gain_db, int enable_adaptive);
void          sonora_agc2_process(sonora_handle h, float *data, int n);
void          sonora_agc2_reset(sonora_handle h);
void          sonora_agc2_destroy(sonora_handle h);

/* ---- high-pass filter -------------------------------------------------- */
/* any frame size. supported rates: 16000, 32000, 48000.                    */

sonora_handle sonora_hpf_create(uint32_t sample_rate);
void          sonora_hpf_process(sonora_handle h, float *data, int n);
void          sonora_hpf_reset(sonora_handle h);
void          sonora_hpf_destroy(sonora_handle h);

/* ---- band splitter ----------------------------------------------------- */
/* splits a full-rate frame into frequency bands for NS/AEC3.               */
/* 16kHz: 1 band (passthrough).  32kHz: 2 bands.  48kHz: 3 bands.          */
/* band_len is always 160.  frame_len = band_len * band_count.              */
/*                                                                          */
/* upper_out / upper_in is a flat buffer of (band_count-1)*band_len floats. */
/* at 48kHz with 3 bands: upper[0..159] = band1, upper[160..319] = band2.  */

sonora_handle sonora_bands_create(uint32_t sample_rate);
int           sonora_bands_count(sonora_handle h);
int           sonora_bands_frame_len(sonora_handle h);
int           sonora_bands_band_len(sonora_handle h);
void          sonora_bands_split(sonora_handle h, const float *frame,
                                 float *lower_out, float *upper_out);
void          sonora_bands_merge(sonora_handle h, const float *lower_in,
                                 const float *upper_in, float *frame_out);
void          sonora_bands_reset(sonora_handle h);
void          sonora_bands_destroy(sonora_handle h);

#ifdef __cplusplus
}
#endif

#endif /* SONORA_H */
