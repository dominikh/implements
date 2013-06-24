package main

// This file contains code from the Go distribution.

// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

import (
	"go/build"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var (
	goroot       = filepath.Clean(runtime.GOROOT())
	gorootSrcPkg = filepath.Join(goroot, "src/pkg")
)

var buildContext = build.Default

// matchPattern(pattern)(name) reports whether
// name matches pattern.  Pattern is a limited glob
// pattern in which '...' means 'any string' and there
// is no other special syntax.
func matchPattern(pattern string) func(name string) bool {
	re := regexp.QuoteMeta(pattern)
	re = strings.Replace(re, `\.\.\.`, `.*`, -1)
	// Special case: foo/... matches foo too.
	if strings.HasSuffix(re, `/.*`) {
		re = re[:len(re)-len(`/.*`)] + `(/.*)?`
	}
	reg := regexp.MustCompile(`^` + re + `$`)
	return func(name string) bool {
		return reg.MatchString(name)
	}
}

func matchPackages(pattern string) []string {
	match := func(string) bool { return true }
	if pattern != "all" && pattern != "std" {
		match = matchPattern(pattern)
	}

	have := map[string]bool{
		"builtin": true, // ignore pseudo-package that exists only for documentation
	}
	if !buildContext.CgoEnabled {
		have["runtime/cgo"] = true // ignore during walk
	}
	var pkgs []string

	// Commands
	cmd := filepath.Join(goroot, "src/cmd") + string(filepath.Separator)
	filepath.Walk(cmd, func(path string, fi os.FileInfo, err error) error {
		if err != nil || !fi.IsDir() || path == cmd {
			return nil
		}
		name := path[len(cmd):]
		// Commands are all in cmd/, not in subdirectories.
		if strings.Contains(name, string(filepath.Separator)) {
			return filepath.SkipDir
		}

		// We use, e.g., cmd/gofmt as the pseudo import path for gofmt.
		name = "cmd/" + name
		if have[name] {
			return nil
		}
		have[name] = true
		if !match(name) {
			return nil
		}
		_, err = buildContext.ImportDir(path, 0)
		if err != nil {
			return nil
		}
		pkgs = append(pkgs, name)
		return nil
	})

	for _, src := range buildContext.SrcDirs() {
		if pattern == "std" && src != gorootSrcPkg {
			continue
		}
		src = filepath.Clean(src) + string(filepath.Separator)
		filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
			if err != nil || !fi.IsDir() || path == src {
				return nil
			}

			// Avoid .foo, _foo, and testdata directory trees.
			_, elem := filepath.Split(path)
			if strings.HasPrefix(elem, ".") || strings.HasPrefix(elem, "_") || elem == "testdata" {
				return filepath.SkipDir
			}

			name := filepath.ToSlash(path[len(src):])
			if pattern == "std" && strings.Contains(name, ".") {
				return filepath.SkipDir
			}
			if have[name] {
				return nil
			}
			have[name] = true
			if !match(name) {
				return nil
			}
			_, err = buildContext.ImportDir(path, 0)
			if err != nil && strings.Contains(err.Error(), "no Go source files") {
				return nil
			}
			pkgs = append(pkgs, name)
			return nil
		})
	}
	return pkgs
}
