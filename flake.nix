{
  description = "fumi development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.gopls
            pkgs.gotools
            pkgs.go-tools

            pkgs.nodejs_24
            pkgs.pnpm
            pkgs.typescript

            pkgs.jq
          ];

          shellHook = ''
            echo "fumi devShell: $(go version), node $(node --version), tsc $(tsc --version)"
          '';
        };
      }
    );
}
