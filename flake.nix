{
  description = "gocpp development shell";

  # nixos-26.05 is the newest branch that still has Go 1.25: nixos-unstable
  # removed go_1_25 when Go 1.25 reached end of life. Its default compilers are
  # clang 21 and GCC 15, the versions the plan was tested with.
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs =
    { nixpkgs, ... }:
    let
      # Add a system here to get the shell there too, e.g. "aarch64-linux".
      systems = [ "x86_64-linux" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      devShells = forAllSystems (
        pkgs:
        let
          lib = pkgs.lib;
          llvm = pkgs.llvmPackages_21;

          # GCC 15 is the default compiler here, and the clang wrapper uses its
          # libstdc++. pkgs.gcc15 would be a second build of the same version, so
          # take the default and fail loudly if a nixpkgs update changes it.
          gcc =
            assert lib.assertMsg (
              lib.versions.major pkgs.gcc.version == "15"
            ) "gocpp wants GCC 15, but this nixpkgs has GCC ${pkgs.gcc.version}";
            pkgs.gcc;

          # MinGW-w64 cross compiler: x86_64-w64-mingw32-gcc/g++ plus its binutils.
          # It targets msvcrt.dll (__MSVCRT_VERSION__=0x600), not UCRT like llvm-mingw
          # and MSYS2 UCRT64: pkgsCross.ucrt64 isn't in the binary cache, so its whole
          # toolchain would build from source.
          # nixpkgs builds it with the mcf thread model. Its thread library
          # (mcfgthread, needed by every libstdc++ header) normally arrives through
          # the wrapper's setup hook, so its paths are written into the wrapper
          # instead, together with winpthreads for <pthread.h> (link with -pthread).
          mingwPkgs = pkgs.pkgsCross.mingwW64;
          mingwThreads = [
            mingwPkgs.threads.package
            mingwPkgs.windows.pthreads
          ];
          mingw = mingwPkgs.buildPackages.gcc.override {
            nixSupport = {
              cc-cflags = map (p: "-isystem ${lib.getDev p}/include") mingwThreads;
              cc-ldflags = map (p: "-L${lib.getLib p}/lib") mingwThreads;
            };
          };
          # Only bin/ goes into the shell. Listed as a package, the wrapper's setup
          # hook would make it the shell's own compiler (CC, CXX, NIX_CC) and hand it
          # the Linux include and library paths of the libraries below.
          # The smoke compile makes a nixpkgs update that moves the thread libraries
          # fail at `nix develop` instead of at the first Windows build.
          mingwSmoke = pkgs.writeText "mingw-smoke.cpp" ''
            #include <pthread.h>
            #include <thread>
            int main() { std::thread t([] { (void)pthread_self(); }); t.join(); }
          '';
          mingwBin = pkgs.runCommandLocal "mingw-w64-gcc-${mingw.version}" { } ''
            ${mingw}/bin/x86_64-w64-mingw32-g++ -std=c++20 -static -pthread ${mingwSmoke} -o smoke.exe
            mkdir -p $out/bin
            ln -s ${mingw}/bin/* $out/bin/
          '';

          # Windows test builds (Phase 06 on): the cross compiler and a 64-bit
          # Wine to run the .exe files. Wine 11 names its binary `wine`.
          windowsTools = lib.optionals pkgs.stdenv.hostPlatform.isx86_64 [
            mingwBin
            pkgs.wine64Packages.stable
          ];
        in
        {
          default = pkgs.mkShell {
            packages = [
              pkgs.go_1_25 # go, gofmt
              # With two compilers, `cc` and `c++` come from the first one listed
              # (clang) and $CC, $CXX from the last one (gcc). Call clang++ or g++
              # by name to get a specific one.
              llvm.clang # clang, clang++
              llvm.clang-tools # clang-format, clangd, clang-tidy
              gcc # gcc, g++
              pkgs.gdb
              llvm.lldb
              pkgs.cmake
              pkgs.ninja
              pkgs.pkg-config
              pkgs.python3 # gdb pretty printers, spike scripts
            ]
            ++ windowsTools;

            # C/C++ libraries for the interop examples and the header zoo.
            buildInputs = [
              pkgs.SDL2
              pkgs.raylib
              pkgs.sqlite
              pkgs.zlib
            ];

            # Only the libraries above (not the tools) add -isystem/-L paths to the
            # compiler wrappers' NIX_CFLAGS_COMPILE and NIX_LDFLAGS.
            strictDeps = true;

            # Outside a Nix build or Nix shell the compiler wrappers add no hardening
            # flags, so turn them off here too (see the plan, Phase 02 §2.7). Otherwise
            # a missing -O becomes -O2, every -O0 file warns about _FORTIFY_SOURCE, and
            # GCC rejects printf(s) with no arguments.
            hardeningDisable = [ "all" ];

            # Use this Go even if a go.mod asks for a newer one, instead of
            # downloading that toolchain.
            GOTOOLCHAIN = "local";

            # Keep Wine from adding menu entries, MIME types and icons to ~/.local/share
            # (whatever WINEPREFIX is). A script that sets WINEDLLOVERRIDES must keep it.
            WINEDLLOVERRIDES = "winemenubuilder.exe=d";
          };
        }
      );
    };
}
