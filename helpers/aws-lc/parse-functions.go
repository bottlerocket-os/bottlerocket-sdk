// Adapted from the Go project's build-goboring.sh:
//   https://github.com/golang/go/blob/master/src/crypto/internal/boring/build-goboring.sh

// Original Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Modifications Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var funcDefinition = regexp.MustCompile("^(const )?[^ ]+ \\**_goboringcrypto_([^(]*)\\(")

var osWriteFile = os.WriteFile

// main is responsible for parsing out the functions from the goboringcrypto.h file.
// It writes it out in several formats to be used later for processing.
func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage: ", os.Args[0], " <path_to_goboringcrypto.h> <output_dir>")
		os.Exit(1)
	}

	if err := writeFunctions(os.Args[1], os.Args[2]); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func writeFunctions(headerFile string, outputDir string) error {
	syms, globals, renames, err := parse(headerFile)
	if err != nil {
		return err
	}

	// Write out our symbols file, ensuring it ends with a newline to prevent some unix tools from failing.
	if err := osWriteFile(filepath.Join(outputDir, "syms.txt"), []byte(strings.Join(syms, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("failed writing syms.txt: %w", err)
	}
	// Write out our globals file, ensuring it ends with a newline to prevent some unix tools from failing.
	if err := osWriteFile(filepath.Join(outputDir, "globals.txt"), []byte(strings.Join(globals, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("failed writing globals.txt: %w", err)
	}
	// Write out our renames file, ensuring it ends with a newline to prevent some unix tools from failing.
	if err := osWriteFile(filepath.Join(outputDir, "renames.txt"), []byte(strings.Join(renames, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("failed writing renames.txt: %w", err)
	}

	return nil
}

func parse(headerFile string) ([]string, []string, []string, error) {
	// Load our header file and break it into lines for processing.
	data, err := os.ReadFile(headerFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed reading %q: %w", headerFile, err)
	}

	// Locate the functions from the header file that we are interested in.
	syms, err := locateFunctions(strings.Split(string(data), "\n"))
	if err != nil {
		return nil, nil, nil, err
	}

	// Add extra functions that aren't in the header.
	syms = append(syms, "BORINGSSL_bcm_power_on_self_test")
	globals, renames := generateGlobalsRenames(syms)

	return syms, globals, renames, nil
}

func generateGlobalsRenames(syms []string) ([]string, []string) {
	globals := make([]string, 0, len(syms))
	renames := make([]string, 0, len(syms)+2)
	for _, sym := range syms {
		globals = append(globals, "_goboringcrypto_"+sym)
		renames = append(renames, sym+" _goboringcrypto_"+sym)
	}

	// Add the functions that we manually compile in.
	renames = append(renames, "__umodti3 _goboringcrypto___umodti3", "__udivti3 _goboringcrypto___udivti3")

	return globals, renames
}

func locateFunctions(lines []string) ([]string, error) {
	funcs := make([]string, 0, len(lines))
	for _, line := range lines {
		match := funcDefinition.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		funcs = append(funcs, match[2])
	}

	return funcs, nil
}
