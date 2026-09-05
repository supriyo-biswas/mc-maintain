// Copyright (c) 2015-2026 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Package quick loads and saves versioned JSON configuration files.
package quick

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"

	"github.com/cheggaaa/pb"
)

// Config provides access to a versioned configuration value.
type Config interface {
	Version() string
	Save(string) error
	Load(string) error
	Data() any
}

type config struct {
	data any
	lock sync.RWMutex
}

// NewConfig initializes a versioned configuration value.
func NewConfig(data any) (Config, error) {
	if err := checkData(data); err != nil {
		return nil, err
	}
	return &config{data: data}, nil
}

// LoadConfig initializes and loads a versioned configuration from filename.
func LoadConfig(filename string, data any) (Config, error) {
	cfg, err := NewConfig(data)
	if err != nil {
		return nil, err
	}
	return cfg, cfg.Load(filename)
}

// Version returns the configuration file format version.
func (c *config) Version() string {
	return reflect.Indirect(reflect.ValueOf(c.data)).FieldByName("Version").String()
}

// Data returns the underlying configuration value.
func (c *config) Data() any {
	return c.data
}

// Load reads JSON configuration from filename into the configuration value.
func (c *config) Load(filename string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	}
	return unmarshalJSON(data, c.data)
}

// Save writes JSON configuration to filename, backing up an existing file to filename.old.
func (c *config) Save(filename string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	oldData, err := os.ReadFile(filename)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else if err = writeFile(filename+".old", oldData); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c.data, "", "\t")
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		data = bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
	}
	return writeFile(filename, data)
}

func checkData(data any) error {
	typ := reflect.TypeOf(data)
	if typ == nil {
		return fmt.Errorf("interface must be struct type")
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return fmt.Errorf("interface must be struct type")
	}
	field, ok := typ.FieldByName("Version")
	if !ok {
		return fmt.Errorf("struct ‘%s’ must have field ‘Version’", typ.Name())
	}
	if field.Type.Kind() != reflect.String {
		return fmt.Errorf("‘Version’ field in struct ‘%s’ must be a string type", typ.Name())
	}
	return nil
}

func unmarshalJSON(data []byte, value any) error {
	err := json.Unmarshal(data, value)
	switch jsonErr := err.(type) {
	case *json.SyntaxError:
		return fmt.Errorf("Unable to parse JSON schema due to a syntax error at '%s'", formatJSONSyntaxError(bytes.NewReader(data), jsonErr.Offset))
	case *json.UnmarshalTypeError:
		return fmt.Errorf("Unable to parse JSON, type '%v' cannot be converted into the Go '%v' type", jsonErr.Value, jsonErr.Type)
	default:
		return err
	}
}

const errorFmt = "%5d: %s  <<<<"

func formatJSONSyntaxError(data io.Reader, offset int64) string {
	var line bytes.Buffer
	lineNumber := 1
	var readBytes int64
	reader := bufio.NewReader(data)
	termWidth := 25
	if width, err := pb.GetTerminalWidth(); err == nil {
		termWidth = width
	}
	errorShift := len(fmt.Sprintf(errorFmt, 1, ""))

readLoop:
	for {
		b, err := reader.ReadByte()
		if err != nil {
			break
		}
		readBytes++
		if readBytes > offset {
			break
		}
		switch b {
		case '\n':
			line.Reset()
			lineNumber++
			continue
		case '\t':
			line.WriteByte(' ')
		case '\r':
			break readLoop
		default:
			line.WriteByte(b)
		}
	}

	lineLength := line.Len()
	start := lineLength - termWidth + errorShift
	if start < 0 || start > lineLength-1 {
		start = 0
	}
	return fmt.Sprintf(errorFmt, lineNumber, line.String()[start:])
}

func writeFile(filename string, data []byte) (err error) {
	if err = os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		return err
	}

	file, err := os.CreateTemp(filepath.Dir(filename), "$tmpfile."+filepath.Base(filename)+".")
	if err != nil {
		return err
	}
	temporaryName := file.Name()
	defer func() {
		if err != nil {
			_ = file.Close()
			_ = os.Remove(temporaryName)
		}
	}()

	if err = os.Chmod(temporaryName, 0o600); err != nil {
		return err
	}
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, filename)
}
