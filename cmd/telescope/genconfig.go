// +build config

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"telescope/controller"

	"github.com/BurntSushi/toml"
)

const header = `# You may copy this sample config as start point.

`

func main() {
	var err error
	defer func() {
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}()

	// set default value here
	var config = Config{
		API: controller.Config{
			Port: 3000,
		},
	}

	var buf bytes.Buffer
	buf.WriteString(header)
	encoder := toml.NewEncoder(&buf)
	err = encoder.Encode(config)
	if err != nil {
		err = fmt.Errorf("encoder.Encode: %w", err)
		return
	}

	f, err := os.Create("config.example.toml")
	if err != nil {
		err = fmt.Errorf("os.Create: %w", err)
		return
	}
	defer f.Close()

	_, err = io.Copy(f, &buf)
	if err != nil {
		err = fmt.Errorf("io.Copy: %w", err)
		return
	}
}
