{
  description = "Incipit - a writing workspace built with Go and MyGo";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
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
          packages = with pkgs; [
            go_1_27
            gotools
            golangci-lint
            gopls
            nixpkgs-fmt
            jq
          ];

          # MyGo v0.10.0 declares Go 1.27.3. nixpkgs currently provides 1.27.1,
          # so let the Go command fetch the exact patch toolchain once.
          GOTOOLCHAIN = "go1.27.3+auto";
          CGO_ENABLED = "0";
        };
      }
    );
}

