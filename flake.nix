{
  description = "katharsis: a competitive-programming roguelike for the terminal";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs, ... }:
    let
      # No x86_64-darwin: the pinned nixpkgs no longer evaluates it (26.11
      # dropped it), so listing it only broke `nix flake check`.
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      forAllSystems = f:
        nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      # The judge's compilers, shared by the package and the dev shell: clang
      # 22, not the default 21, whose 21.1.8 ASan runtime hangs before main
      # on macOS 26.5.
      #
      # On Darwin nixpkgs' bintools (as, dsymutil, nm, ...) come from the
      # default LLVM, so clang 22's wrapper alone drags clang 21 and LLVM 21
      # in (~430 MiB): its `as` runs clang 21, its dsymutil is LLVM 21's,
      # and the Apple SDK links some of those tools into its toolchain dir.
      # Rewiring bintools and the SDK onto LLVM 22 compiles nothing: both
      # are links and wrappers. ld64 and cctools still link LLVM 21's
      # libLTO; rebuilding them on LLVM 22 would save ~380 MiB more for
      # ~6 min of local compiling on every nixpkgs bump.
      #
      # GCC judges what is timed (D28): Apple's libc++ reads ints through
      # cin ~25x slower than libstdc++, so solutions that pass on the
      # Codeforces and USACO g++ judges TLE under clang. It is GCC 14, the
      # major version of Codeforces' newest g++ (14.2), so what compiles
      # here compiles there (GCC 15 adds std::flat_map, for one). GCC ships no
      # sanitizer runtimes for arm64 macOS, so ASan builds stay on clang,
      # compiled against GCC's libstdc++ (clang++-libstdc++) so that every
      # build sees the same headers.
      toolchainFor = pkgs:
        let
          llvm = pkgs.llvmPackages_22;
          gcc = pkgs.gcc14;
        in
        if !pkgs.stdenv.hostPlatform.isDarwin then {
          inherit (llvm) clang;
          inherit gcc;
          gccCc = gcc.cc;
        } else
          let
            binutils-unwrapped = pkgs.darwin.binutils-unwrapped.override {
              llvm = llvm.llvm;
              clang-unwrapped = llvm.clang-unwrapped;
            };
            # NOTE: a no-op, not an error, if nixpkgs stops linking bintools
            #       into the SDK; the closure would just grow again.
            sdk = pkgs.apple-sdk.overrideAttrs (old: {
              postInstall = builtins.replaceStrings
                [ "${pkgs.darwin.binutils-unwrapped}" ]
                [ "${binutils-unwrapped}" ]
                old.postInstall;
            });
            bintools = pkgs.darwin.binutils.override {
              bintools = binutils-unwrapped;
              apple-sdk = sdk;
            };
            # GCC's fixincludes copied the stock SDK's math.h, store path
            # and all, so GCC as built would bring that SDK and LLVM 21
            # back; repointing the reference is a copy, not a GCC build.
            # replaceDependencies returns a bare derivation, and the wrapper
            # reads the rest (isGNU, langCC, lib, ...) from the original.
            gccOut = pkgs.replaceDependencies {
              drv = gcc.cc;
              replacements = [
                { oldDependency = pkgs.apple-sdk; newDependency = sdk; }
              ];
            };
            gccCc = gcc.cc // {
              inherit (gccOut) outPath drvPath;
              out = gccCc;
            };
          in
          {
            inherit gccCc;
            clang = llvm.clang.override {
              inherit bintools;
              apple-sdk = sdk;
            };
            gcc = pkgs.wrapCCWith {
              cc = gccCc;
              inherit bintools;
              apple-sdk = sdk;
            };
            # clang -g on Darwin links through dsymutil, and the wrapper
            # points xcrun at nix's SDK, so /usr/bin/dsymutil fails there
            # ("tool 'dsymutil' not found") and every compile would exit 1.
            dsymutil = "${binutils-unwrapped}/bin/dsymutil";
          };

      # Only the judge's tools go on the game's PATH, not all of cc/bin (cc,
      # ld, strip, ...): a tmux server the game starts inherits that PATH,
      # and the player's own shells and vim run in it.
      # g++ on this PATH also shadows /usr/bin/g++ (Apple clang) in those
      # shells; the judge checks that the g++ it finds is GCC.
      judgeToolsFor = pkgs:
        let
          inherit (pkgs) lib;
          tc = toolchainFor pkgs;
          gccInclude = "${tc.gccCc}/include/c++/${tc.gccCc.version}";
          triple = pkgs.stdenv.hostPlatform.config;
          # Off Darwin, nixpkgs' clang already uses GCC's libstdc++.
          clangLibstdcxx = pkgs.writeShellScript "clang++-libstdc++" (
            if pkgs.stdenv.hostPlatform.isDarwin then ''
              exec ${tc.clang}/bin/clang++ -nostdinc++ \
                -isystem ${gccInclude} \
                -isystem ${gccInclude}/${triple} \
                -isystem ${gccInclude}/backward \
                "$@" -nostdlib++ -L${lib.getLib tc.gccCc}/lib -lstdc++
            '' else ''
              exec ${tc.clang}/bin/clang++ "$@"
            '');
        in
        pkgs.runCommand "katharsis-judge-tools" { } (''
          mkdir -p $out/bin
          ln -s ${tc.clang}/bin/clang++ $out/bin/clang++
          ln -s ${tc.gcc}/bin/g++ $out/bin/g++
          ln -s ${clangLibstdcxx} $out/bin/clang++-libstdc++
        '' + lib.optionalString pkgs.stdenv.hostPlatform.isDarwin ''
          # The include dirs clang++-libstdc++ names must exist.
          test -f ${gccInclude}/${triple}/bits/stdc++.h
          ln -s ${tc.dsymutil} $out/bin/dsymutil
        '');
    in
    {
      packages = forAllSystems (pkgs:
        let
          inherit (pkgs) lib;
        in
        {
          default = pkgs.buildGoModule {
            pname = "katharsis";
            version = "0.1.0";
            src = lib.fileset.toSource {
              root = ./.;
              fileset = lib.fileset.unions [
                ./go.mod
                ./go.sum
                ./cmd
                ./internal
                # The example preset, which the command's tests load.
                (lib.fileset.maybeMissing ./presets)
              ];
            };
            # NOTE: changes whenever go.mod, go.sum or the module's imports do;
            #       `nix build .#default.goModules --rebuild` prints the new
            #       hash.
            vendorHash = "sha256-YiCm0qtBtBgI9GhAdwm9bWQFfzVlXf/+YFJuFRz/mFM=";
            subPackages = [ "cmd/katharsis" ];
            env.CGO_ENABLED = 0;
            ldflags = [ "-s" "-w" ];
            nativeBuildInputs = [ pkgs.makeWrapper ];
            # NOTE: tmux, vim and lldb stay the user's (see the dev shell).
            postInstall = ''
              wrapProgram $out/bin/katharsis --prefix PATH : ${judgeToolsFor pkgs}/bin
            '';
            # The check runs the tests of subPackages only, this command's,
            # which are pure Go and pass offline. The rest of the suite stays
            # out: the judge and session tests skip themselves without clang
            # and tmux, so in the sandbox they would pass having judged
            # nothing, and the engine's balance tests are statistical. Run
            # `go test ./...` in the dev shell for those.
            doCheck = true;
            meta = {
              description = "A competitive-programming roguelike for the terminal";
              mainProgram = "katharsis";
            };
          };
        });

      apps = forAllSystems (pkgs: {
        default = {
          type = "app";
          program = pkgs.lib.getExe self.packages.${pkgs.stdenv.hostPlatform.system}.default;
          meta.description = "Play katharsis";
        };
      });

      devShells = forAllSystems (pkgs: {
        # The judge's tools, exactly as the game's PATH has them, and no
        # stdenv compiler: a cc stdenv's setup hooks export NIX_CFLAGS_COMPILE
        # and friends, which every nix compiler wrapper reads, so tests in the
        # shell would judge with flags the game never passes. It would also
        # bring a second clang and SDK (~430 MiB).
        default = (pkgs.mkShell.override { stdenv = pkgs.stdenvNoCC; }) {
          packages = [
            (judgeToolsFor pkgs)
            pkgs.go
            pkgs.gopls
            pkgs.golangci-lint
          ];
          # NOTE: tmux, vim and lldb come from the user's PATH on purpose. tmux
          #       must talk to the already-running server, vim should load
          #       ~/.vim, and /usr/bin/lldb is what `make lldb` in ~/cp uses.
        };
      });
    };
}
