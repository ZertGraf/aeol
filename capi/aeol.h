/*
 * aeol C API
 *
 * independent audio processing stages from the aeol Go library,
 * exposed as a C shared library.
 *
 * all stages operate on FloatS16 data: float samples in [-32768, 32767].
 * each handle is single-channel. create one handle per channel.
 * handles are NOT thread-safe — protect concurrent access externally.
 *
 * build:
 *   go build -buildmode=c-shared -o aeol.dll  ./capi/   (windows)
 *   go build -buildmode=c-shared -o libaeol.so ./capi/   (linux)
 *   go build -buildmode=c-shared -o libaeol.dylib ./capi/ (macos)
 */

#ifndef AEOL_H
#define AEOL_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef uintptr_t aeol_handle;

/* ---- noise suppressor -------------------------------------------------- */
/* operates on 160-sample frames (10ms at 16kHz).                           */
/* level: 0=low, 1=moderate, 2=high, 3=very_high                           */

aeol_handle aeol_ns_create(int level);
void          aeol_ns_process(aeol_handle h, float *data, int n);
void          aeol_ns_process_upper_band(aeol_handle h, float *data, int n, int band);
void          aeol_ns_reset(aeol_handle h);
void          aeol_ns_destroy(aeol_handle h);

/* ---- echo canceller (aec3) --------------------------------------------- */
/* processes 64-sample blocks internally; pass any multiple of 64.          */
/* call process_render BEFORE process_capture for each frame pair.          */

aeol_handle aeol_aec3_create(uint32_t sample_rate);
void          aeol_aec3_process_render(aeol_handle h, float *data, int n);
void          aeol_aec3_process_capture(aeol_handle h, float *data, int n);
float         aeol_aec3_erle(aeol_handle h);
int           aeol_aec3_delay(aeol_handle h);
void          aeol_aec3_reset(aeol_handle h);
void          aeol_aec3_destroy(aeol_handle h);

/* ---- automatic gain control (agc2) ------------------------------------- */
/* full-band, any frame size. default config: adaptive+fixed gain.          */

aeol_handle aeol_agc2_create(void);
aeol_handle aeol_agc2_create_ex(float fixed_gain_db, int enable_adaptive);
void          aeol_agc2_process(aeol_handle h, float *data, int n);
void          aeol_agc2_reset(aeol_handle h);
void          aeol_agc2_destroy(aeol_handle h);

/* ---- high-pass filter -------------------------------------------------- */
/* any frame size. supported rates: 16000, 32000, 48000.                    */

aeol_handle aeol_hpf_create(uint32_t sample_rate);
void          aeol_hpf_process(aeol_handle h, float *data, int n);
void          aeol_hpf_reset(aeol_handle h);
void          aeol_hpf_destroy(aeol_handle h);

/* ---- band splitter ----------------------------------------------------- */
/* splits a full-rate frame into frequency bands for NS/AEC3.               */
/* 16kHz: 1 band (passthrough).  32kHz: 2 bands.  48kHz: 3 bands.          */
/* band_len is always 160.  frame_len = band_len * band_count.              */
/*                                                                          */
/* upper_out / upper_in is a flat buffer of (band_count-1)*band_len floats. */
/* at 48kHz with 3 bands: upper[0..159] = band1, upper[160..319] = band2.  */

aeol_handle aeol_bands_create(uint32_t sample_rate);
int           aeol_bands_count(aeol_handle h);
int           aeol_bands_frame_len(aeol_handle h);
int           aeol_bands_band_len(aeol_handle h);
void          aeol_bands_split(aeol_handle h, const float *frame,
                                 float *lower_out, float *upper_out);
void          aeol_bands_merge(aeol_handle h, const float *lower_in,
                                 const float *upper_in, float *frame_out);
void          aeol_bands_reset(aeol_handle h);
void          aeol_bands_destroy(aeol_handle h);

#ifdef __cplusplus
}
#endif

#endif /* AEOL_H */
