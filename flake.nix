{
  description = "dead-cases: a competitive-programming roguelike for the terminal";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { nixpkgs, ... }:
    let
      systems = [
        "aarch64-darwin"
        "x86_64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      forAllSystems = f:
        nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
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
