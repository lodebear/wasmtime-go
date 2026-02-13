#!/bin/bash

wasmtime=$1
if [ "$wasmtime" = "" ]; then
  echo "must pass path to wasmtime"
  exit 1
fi

# Clean and re-create "build" directory hierarchy
rm -rf build
for d in "include" "include/wasmtime" "linux-x86_64" "macos-x86_64" "windows-x86_64" "linux-aarch64" "macos-aarch64"; do
  path="build/$d"
  mkdir -p "$path"
  name=$(basename $d)
  echo "package ${name/-/_}" > "$path/empty.go"
done

build="$wasmtime/target/release"
if [ ! -d "$build" ]; then
  build="$wasmtime/target/debug"
fi
# Use absolute path for symbolic links
build=$(cd "$build" && pwd)

if [ ! -f "$build/libwasmtime.a" ]; then
  echo 'Missing libwasmtime.a. Did you `cargo build -p wasmtime-c-api`?'
fi

for d in "linux-x86_64" "macos-x86_64" "linux-aarch64" "macos-aarch64"; do
  ln -s "$build/libwasmtime.a" "build/$d/libwasmtime.a"
done

cp "$wasmtime"/crates/c-api/include/*.h build/include
cp -r "$wasmtime"/crates/c-api/include/wasmtime build/include
if compgen -G "$wasmtime/crates/c-api/wasm-c-api/include/*.h" > /dev/null; then
  cp "$wasmtime"/crates/c-api/wasm-c-api/include/*.h build/include
fi

generated_include=$(find "$wasmtime/target/release/build" "$wasmtime/target/debug/build" -type d -path "*/wasmtime-c-api-impl-*/out/include" 2>/dev/null | head -n 1)
if [ -n "$generated_include" ]; then
  cp -R "$generated_include"/* build/include/
fi

if [ ! -f build/include/wasmtime/conf.h ]; then
  echo "Missing generated header build/include/wasmtime/conf.h"
  exit 1
fi
