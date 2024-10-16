package ipstack

import (
	"bufio"
	"fmt"
	"os"
)

// This file will define all of the functions needed for the REPL functionality

// https://brown-csci1680.github.io/iptcp-docs/specs/repl-commands/

func (s *IPStack) REPL() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)

		//TODO: Implement REPL
	}
}
