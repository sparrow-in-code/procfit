{
  description = "procfit — a low-overhead Linux process observer and workload controller";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    # procfit is Linux-only (it reads /proc and cgroup v2); build for Linux systems.
    flake-utils.lib.eachSystem [ "x86_64-linux" "aarch64-linux" ] (system:
      let
        pkgs = import nixpkgs { inherit system; };
        version = "0.1.0";
        rev = self.rev or self.dirtyRev or "dirty";
        pkg = "github.com/netikras/procfit";
        procfit = pkgs.buildGoModule {
          pname = "procfit";
          inherit version;
          src = self;

          # Update on dependency changes: `nix build` prints the expected hash.
          vendorHash = "sha256-RLuZaaM1PTjP0TL46JhfJi212bFafpqih2oUHLBQOYc=";

          subPackages = [ "cmd/procfit" ];

          # Pure Go (cilium/ebpf; the BPF object is embedded), so a static,
          # cgo-free binary — matches the release artifacts.
          env.CGO_ENABLED = "0";

          ldflags = [
            "-s"
            "-w"
            "-X ${pkg}/internal/meta.Version=${version}"
            "-X ${pkg}/internal/meta.Commit=${builtins.substring 0 12 rev}"
          ];

          # Unit tests need /proc; skip them in the sandbox (CI runs the full gate).
          doCheck = false;

          meta = with pkgs.lib; {
            description = "Low-overhead Linux process explorer, sampler, and workload controller";
            homepage = "https://github.com/sparrow-in-code/procfit";
            license = licenses.mit;
            mainProgram = "procfit";
            platforms = platforms.linux;
          };
        };
      in
      {
        packages.default = procfit;
        packages.procfit = procfit;

        apps.default = {
          type = "app";
          program = "${procfit}/bin/procfit";
        };

        # `nix develop` — the toolchain from QUESTIONS.md (go, make, lint, gcc for
        # -race, clang for `make bpf`).
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [ go gnumake golangci-lint gcc clang ];
        };
      });
}
