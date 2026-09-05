// Copyright (c) 2015-2025 MinIO, Inc.
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
	"github.com/minio/cli"
	json "github.com/minio/colorjson"
	"github.com/minio/mc/pkg/probe"
)

var accesskeySTSRevokeFlags = []cli.Flag{
	cli.BoolFlag{
		Name:  "all",
		Usage: "revoke all STS accounts for the specified user",
	},
	cli.BoolFlag{
		Name:  "self",
		Usage: "revoke all STS accounts for the authenticated user",
	},
	cli.StringFlag{
		Name:  "token-type",
		Usage: "specify the token type to revoke",
	},
}

type stsRevokeMessage struct {
	Status          string `json:"status"`
	User            string `json:"user"`
	TokenRevokeType string `json:"tokenRevokeType,omitempty"`
}

func (m stsRevokeMessage) String() string {
	userString := "user " + m.User
	if m.User == "" {
		userString = "authenticated user"
	}
	if m.TokenRevokeType == "" {
		return "Successfully revoked all STS accounts for " + userString
	}
	return "Successfully revoked all STS accounts of type " + m.TokenRevokeType + " for " + userString
}

func (m stsRevokeMessage) JSON() string {
	if m.Status == "" {
		m.Status = "success"
	}
	jsonMessageBytes, e := json.MarshalIndent(m, "", " ")
	fatalIf(probe.NewError(e), "Unable to marshal into JSON.")

	return string(jsonMessageBytes)
}

// checkSTSRevokeSyntax - validate all the passed arguments
func checkSTSRevokeSyntax(ctx *cli.Context) {
	if len(ctx.Args()) > 2 || len(ctx.Args()) == 0 {
		cli.ShowCommandHelpAndExit(ctx, ctx.Command.Name, 1)
	}

	if !ctx.Bool("self") && ctx.Args().Get(1) == "" {
		fatalIf(errInvalidArgument().Trace(), "Must specify user or use --self flag.")
	}

	if ctx.Bool("self") && ctx.Args().Get(1) != "" {
		fatalIf(errInvalidArgument().Trace(), "Cannot specify user with --self flag.")
	}

	if (!ctx.Bool("all") && ctx.String("token-type") == "") || (ctx.Bool("all") && ctx.String("token-type") != "") {
		fatalIf(errDummy().Trace(), "Exactly one of --all or --token-type must be specified.")
	}
}
