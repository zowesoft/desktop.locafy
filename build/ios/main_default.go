//go:build !ios

// Package main holds the iOS glue for the application. It is built as a C
// archive with `-tags ios` (see Taskfile.yml), which needs no entry point.
//
// On a desktop platform only this file of the package compiles, and a main
// package without a main function fails the build, which made `go build ./...`
// report "function main is undeclared". This stub is never run: the package is
// not the application's entry point, and it is never executed on desktop. It
// exists so the whole module builds, which is what CI checks.
package main

func main() {}
