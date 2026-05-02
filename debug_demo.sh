#!/bin/bash
docker run --rm --platform linux/arm64 -v /Volumes/External/Code/VMPacker:/work -w /work/demo vmp-builder gdb -batch -ex 'run' -ex 'bt' -ex 'info registers' ./demo_complex.vmp
