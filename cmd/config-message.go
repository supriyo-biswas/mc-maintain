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
	"fmt"

	"github.com/fatih/color"
	json "github.com/minio/colorjson"
	"github.com/minio/mc/pkg/probe"
	"github.com/minio/pkg/v3/console"
)

// configSetMessage container to hold locks information.
type configSetMessage struct {
	Status      string `json:"status"`
	targetAlias string
	restart     bool
}

// String colorized service status message.
func (u configSetMessage) String() (msg string) {
	msg += console.Colorize("SetConfigSuccess",
		"Successfully applied new settings.")
	if u.restart {
		suggestion := color.RedString(u.targetAlias)
		msg += console.Colorize("SetConfigSuccess",
			fmt.Sprintf("\nPlease restart your server '%s'.", suggestion))
	}
	return
}

// JSON jsonified service status message.
func (u configSetMessage) JSON() string {
	u.Status = "success"
	statusJSONBytes, e := json.MarshalIndent(u, "", " ")
	fatalIf(probe.NewError(e), "Unable to marshal into JSON.")

	return string(statusJSONBytes)
}
