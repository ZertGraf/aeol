// amalgamation: compile all rnnoise sources as a single translation unit.
// order matters — denoise.c depends on the others.
#include "../third_party/rnnoise/src/celt_lpc.c"
#include "../third_party/rnnoise/src/kiss_fft.c"
#include "../third_party/rnnoise/src/pitch.c"
#include "../third_party/rnnoise/src/rnn.c"
#include "../third_party/rnnoise/src/rnn_data.c"
#include "../third_party/rnnoise/src/denoise.c"
