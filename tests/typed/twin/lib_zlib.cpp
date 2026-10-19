// The C++ twin of lib_zlib.gp: the expected output is what this program prints.
#include <unistd.h>
#include <zlib.h>
#include <cstdio>
#include <string>

int main() {
    std::string path = "/tmp/gocpp-typed-twin-" + std::to_string(getpid()) + ".gz";
    std::string mode = "wb";
    gzFile gz = gzopen(path.c_str(), mode.c_str());
    std::string text = "hello hello hello\n";
    gzputs(gz, text.c_str());
    gzputs(gz, (text + "second line\n").c_str());
    gzclose(gz);

    gzFile rd = gzopen(path.c_str(), "rb");
    char buf[64];
    int lines = 0;
    while (gzgets(rd, buf, 64) != nullptr) {
        lines++;
        std::fputs(buf, stdout);
    }
    gzclose(rd);
    std::remove(path.c_str());
    std::printf("lines: %d\n", lines);

    std::string src = "hello hello hello hello hello hello hello hello";
    long bound = compressBound(src.size());
    std::printf("%s\n", bound > (long)src.size() ? "true" : "false");
    uLong crc = crc32(0, (const Bytef*)src.data(), (uInt)src.size());
    unsigned sum = crc;
    std::printf("%u %s\n", sum, zError(Z_DATA_ERROR));
}
