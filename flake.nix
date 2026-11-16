{
  description = "mtg-mcp";

  # nixpkgs' default `go` stays on the release's original major; go.mod needs 1.27, so this uses
  # buildGo127Module. Consumers can point this input at their own nixpkgs with `follows`.
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-26.05-darwin";

  outputs = { self, nixpkgs }:
    let
      # x86_64-linux is for CI, which keeps vendorHash current on ubuntu runners.
      systems = [ "aarch64-darwin" "x86_64-linux" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      packages = forAllSystems (pkgs: {
        default = pkgs.buildGo127Module rec {
          pname = "mtg-mcp";
          version = self.shortRev or self.dirtyShortRev or "dev";
          src = self;
          subPackages = [ "cmd/mtg-mcp" ];
          # Replace with the hash the first `nix build` prints; update it when go.sum changes.
          vendorHash = "sha256-YSiKPgMRnEgGikZixmhZ7sY1K7ID2o8zSM0nuJinIu8=";
          # The default name includes the version (the commit here), which would re-download every
          # module on each new commit; keyed on pname alone they are reused until vendorHash changes.
          overrideModAttrs = _: { name = "${pname}-go-modules"; };
          env.CGO_ENABLED = 0;
          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
            "-X main.commit=${self.rev or self.dirtyRev or "none"}"
            "-X main.buildDate=${self.lastModifiedDate or "unknown"}"
          ];
          # Some tests need the network, which the build sandbox doesn't have.
          doCheck = false;
          meta.mainProgram = "mtg-mcp";
        };
      });
    };
}
