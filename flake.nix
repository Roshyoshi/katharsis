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
    in
    {
      packages = forAllSystems (pkgs:
        let
          inherit (pkgs) lib;
          # The judge compiler: clang 22, as in the dev shell.
          cc = pkgs.llvmPackages_22.clang;
          # Only the judge's tools go on the game's PATH, not all of cc/bin
          # (cc, ld, strip, ...): a tmux server the game starts inherits that
          # PATH, and the player's own shells and vim run in it. clang -g on
          # Darwin links through dsymutil, and the wrapper points xcrun at
          # nix's SDK, so /usr/bin/dsymutil fails there ("tool 'dsymutil'
          # not found") and every compile would exit 1; the dev shell gets
          # this same dsymutil from its stdenv.
          judgeTools = pkgs.runCommand "katharsis-judge-tools" { } (''
            mkdir -p $out/bin
            ln -s ${cc}/bin/clang++ $out/bin/clang++
          '' + lib.optionalString pkgs.stdenv.hostPlatform.isDarwin ''
            ln -s ${cc.bintools.bintools}/bin/dsymutil $out/bin/dsymutil
          '');
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
            vendorHash = "sha256-gvArVMQLDbazowHwP56tEjVBgbKZ1MuicWcs3uaeUHg=";
            subPackages = [ "cmd/katharsis" ];
            env.CGO_ENABLED = 0;
            ldflags = [ "-s" "-w" ];
            nativeBuildInputs = [ pkgs.makeWrapper ];
            # NOTE: tmux, vim and lldb stay the user's (see the dev shell).
            postInstall = ''
              wrapProgram $out/bin/katharsis --prefix PATH : ${judgeTools}/bin
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
        # The shell's own cc is the judge compiler. clang 22, not the default
        # 21: 21.1.8's ASan runtime hangs before main on macOS 26.5.
        default = (pkgs.mkShell.override { stdenv = pkgs.llvmPackages_22.stdenv; }) {
          packages = [
            pkgs.go
            pkgs.gopls
            pkgs.golangci-lint
          ];
          # nix's hardening adds -fwrapv (UBSan then skips signed overflow)
          # and libc++'s fast hardening (traps before ASan can report).
          hardeningDisable = [ "all" ];
          # NOTE: tmux, vim and lldb come from the user's PATH on purpose. tmux
          #       must talk to the already-running server, vim should load
          #       ~/.vim, and /usr/bin/lldb is what `make lldb` in ~/cp uses.
        };
      });
    };
}
