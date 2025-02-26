{
  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";

  outputs = { nixpkgs, ... }:
    let
      forSystem = nixpkgs.lib.genAttrs [
        "x86_64-linux"
        "aarch64-darwin"
      ];
      pkgsFor = forSystem (system :
        import nixpkgs { inherit system; }
      );
    in
    {
      devShells = forSystem
        (system:
          let
            pkgs = pkgsFor."${system}";
          in
            {
              default = pkgs.mkShell {
                buildInputs = with pkgs;
                  [
                    go 
                    gopls

                    protobuf
                  ];
                shellHook = ''
                  export PS1="[nix@fenrir] \W § "
                '';
              };
            }
        );
    };
}
