{
  pkgs,
  lib,
  config,
  inputs,
  ...
}:

{
  packages = [
    pkgs.kubernetes-helm
    pkgs.kustomize
  ];

  languages.go.enable = true;
  env.CGO_ENABLED = false;
}
