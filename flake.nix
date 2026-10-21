{
  description = "gocpp: Go-syntax programs compiled to readable C++20 (the package, and the development shell)";

  # nixos-26.05 is the newest branch that still has Go 1.25: nixos-unstable
  # removed go_1_25 when Go 1.25 reached end of life. Its default compilers are
  # clang 21 and GCC 15, the versions the plan was tested with.
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs =
    { self, nixpkgs, ... }:
    let
      # The development shell exists where the project is tested: x86_64-linux (the developer
      # machines) and aarch64-linux (the arm64 CI job, step 15.12). The Windows tools (MinGW-w64
      # and Wine) are x86-64 only and are left out of the arm64 shell (windowsTools below).
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      # The package (nix build, nix run, nix profile install) is built for the systems that have
      # release binaries too (.goreleaser.yaml); only x86_64-linux is tested (step 15.13).
      packageSystems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      forAllPackageSystems =
        f: nixpkgs.lib.genAttrs packageSystems (system: f nixpkgs.legacyPackages.${system});

      # gocpp, wrapped so that the compilers it needs are found on NixOS, where no compiler is
      # on the PATH by default. The runtime and the standard library are embedded in the binary
      # (ADR-009), so the Go program is all there is to build.
      mkGocpp =
        pkgs:
        let
          lib = pkgs.lib;
          llvm = pkgs.llvmPackages_21;

          # Only what the Go program needs to build: no tests, plans, docs or tools. The runtime
          # is embedded from four places of runtime/ (runtime/files.go lists them).
          src = lib.fileset.toSource {
            root = ./.;
            fileset = lib.fileset.unions [
              ./go.mod
              ./go.sum
              ./LICENSE
              ./THIRD_PARTY_NOTICES.txt
              ./cmd
              ./internal
              ./stdlib
              ./runtime/files.go
              ./runtime/include
              ./runtime/src
              ./runtime/debug
            ];
          };

          # The wrapped compilers: Nix's g++ and clang++ add the include and library paths of
          # their own libstdc++ and libc, so a generated program builds and links wherever
          # gocpp runs. binutils has ar and nm (the runtime library, gocpp build -buildmode);
          # llvm-ar is for clang's LTO objects; pkg-config is for interop with C libraries.
          compilers = [
            pkgs.gcc
            llvm.clang
            pkgs.binutils
            llvm.llvm
            pkgs.pkg-config
          ];

          # "dev" plus the commit: a flake build is not a tagged release.
          rev = self.shortRev or self.dirtyShortRev or "unknown";
        in
        (pkgs.buildGoModule.override { go = pkgs.go_1_25; }) {
          pname = "gocpp";
          version = "dev-${rev}";
          inherit src;
          # The hash of the downloaded modules: it changes with go.mod and go.sum. When
          # TestFlakeVendorHash (internal/packaging) says that go.mod or go.sum changed, run
          #   nix build .#gocpp
          # with lib.fakeHash here, copy the hash that Nix reports, and write the new
          # sha256 of go.mod followed by go.sum (cat go.mod go.sum | sha256sum) on the next line.
          # go.mod+go.sum sha256: 266875ca3c94dc999671c0ddd01ac2d3595642481d8796b13d40d64e7b8e71f3
          vendorHash = "sha256-yQpnSIS+F8ANLTm1WdUe1Nc8me1LK98G0gjQQ8vaC6o=";
          subPackages = [ "cmd/gocpp" ];
          # Download the module cache (one dependency, golang.org/x/tools) instead of vendoring:
          # go mod vendor also wants the packages that the tests of cmd/gocpp import, which the
          # source above leaves out (tools/, tests/).
          proxyVendor = true;
          env.CGO_ENABLED = "0";
          ldflags = [
            "-s"
            "-w"
            "-X github.com/huuhait/gocpp/internal/version.Version=dev-${rev}"
          ];

          # The tests of gocpp compile C++ for minutes; they are run in the repository
          # (make ci), not for every package build. The install check below builds and runs a
          # program with the wrapped compilers instead.
          doCheck = false;

          nativeBuildInputs = [ pkgs.makeWrapper ];
          postInstall = ''
            wrapProgram $out/bin/gocpp --prefix PATH : ${lib.makeBinPath compilers}
            install -Dm644 LICENSE $out/share/doc/gocpp/LICENSE
            install -Dm644 THIRD_PARTY_NOTICES.txt $out/share/doc/gocpp/THIRD_PARTY_NOTICES.txt
          '';

          # No hardening flags from the compiler wrappers (see the dev shell).
          hardeningDisable = [ "all" ];

          doInstallCheck = true;
          installCheckPhase = ''
            runHook preInstallCheck
            export HOME=$TMPDIR XDG_CACHE_HOME=$TMPDIR/cache
            mkdir hello && cd hello
            printf 'package main\n\nimport "fmt"\n\nfunc main() { fmt.Println("hello from the nix package") }\n' > hello.gp
            for cxx in g++ clang++; do
              echo "== $cxx"
              GOCPP_CXX=$cxx $out/bin/gocpp run hello.gp | tee out.txt
              grep -q 'hello from the nix package' out.txt
            done
            $out/bin/gocpp version
            runHook postInstallCheck
          '';

          meta = {
            description = "Go-syntax programs compiled to readable C++20";
            homepage = "https://github.com/huuhait/gocpp";
            license = lib.licenses.bsd3;
            mainProgram = "gocpp";
            platforms = lib.platforms.linux;
          };
        };
    in
    {
      packages = forAllPackageSystems (pkgs: rec {
        gocpp = mkGocpp pkgs;
        default = gocpp;
      });

      apps = forAllPackageSystems (pkgs: rec {
        gocpp = {
          type = "app";
          meta.description = "gocpp: Go-syntax programs compiled to readable C++20";
          program = "${self.packages.${pkgs.stdenv.hostPlatform.system}.gocpp}/bin/gocpp";
        };
        default = gocpp;
      });

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
              # Before clang: the clang wrapper brings the unwrapped clang's bin/ into
              # PATH right after its own, and that bin/ has a clangd that can't find
              # the standard headers. clang-tools' clangd is wrapped and finds them.
              llvm.clang-tools # clang-format, clangd, clang-tidy
              # With two compilers, `cc` and `c++` come from the first one listed
              # (clang) and $CC, $CXX from the last one (gcc). Call clang++ or g++
              # by name to get a specific one.
              llvm.clang # clang, clang++
              gcc # gcc, g++
              pkgs.gdb
              llvm.lldb
              pkgs.cmake
              pkgs.ninja
              pkgs.pkg-config
              pkgs.python3 # gdb pretty printers, spike scripts, go generate in internal/diag
              pkgs.nodejs # node, npm: the VS Code grammar tests (make vscode-test)
              # A hidden X display, so GUI programs run without a real one: the
              # raylib example's smoke mode runs under xvfb-run (check.sh, CI).
              pkgs.xvfb-run
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

            # The LLVM tools (opt, llvm-dis) and headers (llvm/Passes/PassPlugin.h) of the tests
            # of `-cxxflags=-fpass-plugin=` and `-emit-llvm` (cmd/gocpp/llvmplugin_test.go). Only
            # these variables name them: the tools are not on PATH, so the build driver still finds
            # the same archiver and linker as before.
            GOCPP_LLVM_BIN = "${llvm.llvm}/bin";
            GOCPP_LLVM_DEV = "${llvm.llvm.dev}";

            # Keep Wine from adding menu entries, MIME types and icons to ~/.local/share
            # (whatever WINEPREFIX is). A script that sets WINEDLLOVERRIDES must keep it.
            WINEDLLOVERRIDES = "winemenubuilder.exe=d";
          };
        }
      );
    };
}
