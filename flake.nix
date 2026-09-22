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
        # Compose the same SemVer + yyyyMMdd-HHmmss-<sha> build id as
        # scripts/version.sh, from the flake's own metadata (hermetic: no git/date
        # calls at build time). lastModifiedDate is "YYYYMMDDHHMMSS".
        shortRev = self.shortRev or self.dirtyShortRev or "unknown";
        lastMod = self.lastModifiedDate or "00000000000000";
        buildDate = "${builtins.substring 0 8 lastMod}-${builtins.substring 8 6 lastMod}";
        fullVersion = "${version}+${buildDate}-${shortRev}";
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
            "-X ${pkg}/internal/meta.Version=${fullVersion}"
            "-X ${pkg}/internal/meta.Commit=${shortRev}"
            "-X ${pkg}/internal/meta.BuildDate=${buildDate}"
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
