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

}
