##
## aarch64 / ARM64 Linux makefile for SCADACS/snap7.
## Upstream does not ship this target; kept in-tree for multi-arch images.
##
TargetCPU  :=aarch64
OS         :=linux
CXXFLAGS   := -std=c++11 -fdiagnostics-color -O3 -fPIC -pedantic

include common.mk
