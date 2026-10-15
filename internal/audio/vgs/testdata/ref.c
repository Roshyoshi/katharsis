/* Reference renderer for TestConformance: compiles each MML file with the
 * C compiler, decodes it in 441-sample calls (as tohovgs does) and prints
 * timing and an FNV-1a 64 hash of the first pass plus 10 s past it. It
 * opens no audio device and plays nothing; DUMP=<file> also writes the
 * raw samples of the last song.
 *
 * Build it against tohovgs-cli/src (cbb13ada5dd1d9244148a265946310bdd192d4b6):
 *   cc -O2 -w -o ref ref.c -I$S $S/vgsmml.c $S/vgsdec.c $S/vgs2tone.c $S/miniz.c
 *   ./ref mml/*.mml > ref.txt
 *   VGS_REF=$PWD/ref.txt go test -run Conformance ./internal/audio/vgs
 */
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include "vgsdec.h"
#include "vgsmml.h"

int main(int argc, char** argv)
{
    void* ctx = vgsdec_create_context();
    for (int a = 1; a < argc; a++) {
        struct VgsMmlErrorInfo err;
        struct VgsBgmData* bgm = vgsmml_compile_from_file(argv[a], &err);
        if (!bgm) {
            printf("%s ERR %d %s\n", argv[a], err.line, err.message);
            continue;
        }
        if (vgsdec_load_bgm_from_memory(ctx, bgm->data, bgm->size)) {
            printf("%s LOADERR\n", argv[a]);
            vgsmml_free_bgm_data(bgm);
            continue;
        }
        vgsmml_free_bgm_data(bgm);
        long timeI = vgsdec_get_value(ctx, VGSDEC_REG_LOOP_TIME);
        long timeL = vgsdec_get_value(ctx, VGSDEC_REG_TIME_LENGTH);
        long loopIdx = vgsdec_get_value(ctx, VGSDEC_REG_LOOP_INDEX);
        long n = timeL + 220500;
        uint64_t h = 14695981039346656037ULL;
        short buf[441];
        long done = 0;
        FILE* dump = getenv("DUMP") ? fopen(getenv("DUMP"), "wb") : NULL;
        int64_t peak = 0;
        while (done < n) {
            vgsdec_execute(ctx, buf, sizeof(buf));
            if (dump) fwrite(buf, 2, 441, dump);
            for (int i = 0; i < 441 && done < n; i++, done++) {
                uint16_t v = (uint16_t)buf[i];
                h ^= (uint8_t)(v & 0xff); h *= 1099511628211ULL;
                h ^= (uint8_t)(v >> 8); h *= 1099511628211ULL;
                int64_t m = buf[i] < 0 ? -(int64_t)buf[i] : buf[i];
                if (m > peak) peak = m;
            }
        }
        if (dump) fclose(dump);
        printf("%s %ld %ld %ld %ld %016llx %lld\n", argv[a], timeI, timeL, loopIdx, n, (unsigned long long)h, (long long)peak);
    }
    vgsdec_release_context(ctx);
    return 0;
}
