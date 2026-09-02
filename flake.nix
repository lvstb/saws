{
  description = "saws - AWS SSO credentials, without the pain";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachSystem [
      "x86_64-linux"
      "aarch64-linux"
      "x86_64-darwin"
      "aarch64-darwin"
    ] (system:
      let
        pkgs = import nixpkgs { inherit system; };
        saws = pkgs.buildGoModule {
          pname = "saws";
          version = "dev";
          src = ./.;
          vendorHash = "sha256-XfFvzGhAoYmx81eiuI3Ee0anNJ06TrzyHJwZXgQxex0=";
          ldflags = [
            "-s"
            "-w"
            "-X main.version=dev"
          ];
        };
      in {
        packages = {
          default = saws;
          saws = saws;
        };

        apps = {
          default = flake-utils.lib.mkApp { drv = saws; };
          saws = flake-utils.lib.mkApp { drv = saws; };
        };

        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.gopls
          ];
        };
      });
}
