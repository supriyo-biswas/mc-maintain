// Copyright (c) 2015-2022 MinIO, Inc.
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

package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/minio/mc/pkg/probe"
)

const (
	// Maximum length for MinIO access keys.
	accessKeyMaxLen = 20

	// Alpha numeric table used for generating access keys.
	alphaNumericTable    = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	alphaNumericTableLen = byte(len(alphaNumericTable))

	// Maximum secret key length for MinIO.
	secretKeyMaxLen = 40
)

var supportedTimeFormats = []string{
	"2006-01-02",
	"2006-01-02T15:04",
	"2006-01-02T15:04:05",
	time.RFC3339,
}

// generateCredentials - creates randomly generated credentials of maximum
// allowed length.
func generateCredentials() (accessKey, secretKey string, err *probe.Error) {
	readBytes := func(size int) (data []byte, e error) {
		data = make([]byte, size)
		var n int
		if n, e = rand.Read(data); e != nil {
			return nil, e
		} else if n != size {
			return nil, fmt.Errorf("Not enough data. Expected to read: %v bytes, got: %v bytes", size, n)
		}
		return data, nil
	}

	// Generate access key.
	keyBytes, e := readBytes(accessKeyMaxLen)
	if e != nil {
		return "", "", probe.NewError(e)
	}
	for i := range accessKeyMaxLen {
		keyBytes[i] = alphaNumericTable[keyBytes[i]%alphaNumericTableLen]
	}
	accessKey = string(keyBytes)

	// Generate secret key.
	keyBytes, e = readBytes(secretKeyMaxLen)
	if e != nil {
		return "", "", probe.NewError(e)
	}

	secretKey = strings.ReplaceAll(string([]byte(base64.StdEncoding.EncodeToString(keyBytes))[:secretKeyMaxLen]),
		"/", "+")

	return accessKey, secretKey, nil
}
