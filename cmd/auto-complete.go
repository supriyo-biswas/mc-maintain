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
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/minio/cli"
	"github.com/posener/complete"
)

// fsComplete knows how to complete file/dir names by the given path
type fsComplete struct{}

// predictPathWithTilde completes an FS path which starts with a `~/`
func (fs fsComplete) predictPathWithTilde(a complete.Args) []string {
	homeDir, e := os.UserHomeDir()
	if e != nil || homeDir == "" {
		return nil
	}
	// Clean the home directory path
	homeDir = strings.TrimRight(homeDir, "/")

	// Replace the first occurrence of ~ with the real path and complete
	a.Last = strings.Replace(a.Last, "~", homeDir, 1)
	predictions := complete.PredictFiles("*").Predict(a)

	// Restore ~ to avoid disturbing the completion user experience
	for i := range predictions {
		predictions[i] = strings.Replace(predictions[i], homeDir, "~", 1)
	}

	return predictions
}

func (fs fsComplete) Predict(a complete.Args) []string {
	if strings.HasPrefix(a.Last, "~/") {
		return fs.predictPathWithTilde(a)
	}
	return complete.PredictFiles("*").Predict(a)
}

// Complete S3 path. If the prediction result is only one directory,
// then recursively scans it. This is needed to satisfy posener/complete
// (look at posener/complete.PredictFiles)
func completeS3Path(s3Path string) (prediction []string) {
	// Convert alias/bucket/incompl to alias/bucket/ to list its contents
	parentDirPath := filepath.Dir(s3Path) + "/"
	clnt, err := newClient(parentDirPath)
	if err != nil {
		return nil
	}

	// Calculate alias from the path
	alias := splitStr(s3Path, "/", 3)[0]

	// List dirPath content and only pick elements that corresponds
	// to the path that we want to complete
	for content := range clnt.List(globalContext, ListOptions{Recursive: false, ShowDir: DirFirst}) {
		cmplS3Path := alias + getKey(content)
		if content.Type.IsDir() {
			if !strings.HasSuffix(cmplS3Path, "/") {
				cmplS3Path += "/"
			}
		}
		if strings.HasPrefix(cmplS3Path, s3Path) {
			prediction = append(prediction, cmplS3Path)
		}
	}

	// If completion found only one directory, recursively scan it.
	if len(prediction) == 1 && strings.HasSuffix(prediction[0], "/") {
		prediction = append(prediction, completeS3Path(prediction[0])...)
	}

	return
}

// s3Complete knows how to complete an mc s3 path
type s3Complete struct {
	deepLevel int
}

func (s3 s3Complete) Predict(a complete.Args) (prediction []string) {
	defer func() {
		sort.Strings(prediction)
	}()

	loadMcConfig = loadMcConfigFactory()
	conf, err := loadMcConfig()
	if err != nil {
		return nil
	}

	arg := a.Last

	if strings.IndexByte(arg, '/') == -1 {
		// Only predict alias since '/' is not found
		for alias := range conf.Aliases {
			if strings.HasPrefix(alias, arg) {
				prediction = append(prediction, alias+"/")
			}
		}
		if len(prediction) == 1 && strings.HasSuffix(prediction[0], "/") {
			prediction = append(prediction, completeS3Path(prediction[0])...)
		}
	} else {
		// Complete S3 path until the specified path deep level
		if s3.deepLevel > 0 {
			if strings.Count(arg, "/") >= s3.deepLevel {
				return []string{arg}
			}
		}
		// Predict S3 path
		prediction = completeS3Path(arg)
	}

	return
}

// aliasComplete only completes aliases
type aliasComplete struct{}

func (al aliasComplete) Predict(a complete.Args) (prediction []string) {
	defer func() {
		sort.Strings(prediction)
	}()

	loadMcConfig = loadMcConfigFactory()
	conf, err := loadMcConfig()
	if err != nil {
		return nil
	}

	arg := a.Last
	for alias := range conf.Aliases {
		if strings.HasPrefix(alias, arg) {
			prediction = append(prediction, alias+"/")
		}
	}

	return
}

var (
	s3Completer    = s3Complete{}
	aliasCompleter = aliasComplete{}
	fsCompleter    = fsComplete{}
)

// The list of all commands supported by mc with their mapping
// with their bash completer function
var completeCmds = map[string]complete.Predictor{
	// S3 API level commands
	"/ls":        complete.PredictOr(s3Completer, fsCompleter),
	"/cp":        complete.PredictOr(s3Completer, fsCompleter),
	"/mv":        complete.PredictOr(s3Completer, fsCompleter),
	"/rm":        complete.PredictOr(s3Completer, fsCompleter),
	"/rb":        complete.PredictOr(s3Complete{deepLevel: 2}, fsCompleter),
	"/cat":       complete.PredictOr(s3Completer, fsCompleter),
	"/head":      complete.PredictOr(s3Completer, fsCompleter),
	"/diff":      complete.PredictOr(s3Completer, fsCompleter),
	"/find":      complete.PredictOr(s3Completer, fsCompleter),
	"/mirror":    complete.PredictOr(s3Completer, fsCompleter),
	"/pipe":      complete.PredictOr(s3Completer, fsCompleter),
	"/stat":      complete.PredictOr(s3Completer, fsCompleter),
	"/watch":     complete.PredictOr(s3Completer, fsCompleter),
	"/anonymous": complete.PredictOr(s3Completer, fsCompleter),
	"/tree":      complete.PredictOr(s3Complete{deepLevel: 2}, fsCompleter),
	"/du":        complete.PredictOr(s3Complete{deepLevel: 2}, fsCompleter),

	"/retention/set":   s3Completer,
	"/retention/clear": s3Completer,
	"/retention/info":  s3Completer,

	"/legalhold/set":   s3Completer,
	"/legalhold/clear": s3Completer,
	"/legalhold/info":  s3Completer,

	"/sql": s3Completer,
	"/mb":  aliasCompleter,

	"/event/add":    s3Complete{deepLevel: 2},
	"/event/list":   s3Complete{deepLevel: 2},
	"/event/remove": s3Complete{deepLevel: 2},

	"/encrypt/set":   s3Complete{deepLevel: 2},
	"/encrypt/info":  s3Complete{deepLevel: 2},
	"/encrypt/clear": s3Complete{deepLevel: 2},

	"/tag/list":   s3Completer,
	"/tag/remove": s3Completer,
	"/tag/set":    s3Completer,

	"/version/info":    s3Complete{deepLevel: 2},
	"/version/enable":  s3Complete{deepLevel: 2},
	"/version/suspend": s3Complete{deepLevel: 2},

	"/lock/compliance": s3Completer,
	"/lock/governance": s3Completer,
	"/lock/clear":      s3Completer,
	"/lock/info":       s3Completer,

	"/share/download": s3Completer,
	"/share/list":     nil,
	"/share/upload":   s3Completer,

	"/ilm/list":    s3Complete{deepLevel: 2},
	"/ilm/add":     s3Complete{deepLevel: 2},
	"/ilm/edit":    s3Complete{deepLevel: 2},
	"/ilm/remove":  s3Complete{deepLevel: 2},
	"/ilm/export":  s3Complete{deepLevel: 2},
	"/ilm/import":  s3Complete{deepLevel: 2},
	"/ilm/restore": s3Completer,

	"/ilm/rule/list":    s3Complete{deepLevel: 2},
	"/ilm/rule/add":     s3Complete{deepLevel: 2},
	"/ilm/rule/edit":    s3Complete{deepLevel: 2},
	"/ilm/rule/remove":  s3Complete{deepLevel: 2},
	"/ilm/rule/export":  s3Complete{deepLevel: 2},
	"/ilm/rule/import":  s3Complete{deepLevel: 2},
	"/ilm/rule/restore": s3Completer,

	"/undo": s3Completer,

	"/alias/set":    nil,
	"/alias/list":   aliasCompleter,
	"/alias/remove": aliasCompleter,
	"/alias/import": nil,
	"/alias/export": aliasCompleter,

	"/put": complete.PredictOr(s3Completer, fsCompleter),
	"/get": complete.PredictOr(s3Completer, fsCompleter),

	"/cors/set":    s3Complete{deepLevel: 2},
	"/cors/get":    s3Complete{deepLevel: 2},
	"/cors/remove": s3Complete{deepLevel: 2},
}

// flagsToCompleteFlags transforms a cli.Flag to complete.Flags
// understood by posener/complete library.
func flagsToCompleteFlags(flags []cli.Flag) complete.Flags {
	complFlags := make(complete.Flags)
	for _, f := range flags {
		for _, s := range strings.Split(f.GetName(), ",") {
			var flagName string
			s = strings.TrimSpace(s)
			if len(s) == 1 {
				flagName = "-" + s
			} else {
				flagName = "--" + s
			}
			complFlags[flagName] = complete.PredictNothing
		}
	}
	return complFlags
}

// This function recursively transforms cli.Command to complete.Command
// understood by posener/complete library.
func cmdToCompleteCmd(cmd cli.Command, parentPath string) complete.Command {
	var complCmd complete.Command
	complCmd.Sub = make(complete.Commands)

	for _, subCmd := range cmd.Subcommands {
		if subCmd.Hidden {
			continue
		}
		complCmd.Sub[subCmd.Name] = cmdToCompleteCmd(subCmd, parentPath+"/"+cmd.Name)
		for _, alias := range subCmd.Aliases {
			complCmd.Sub[alias] = cmdToCompleteCmd(subCmd, parentPath+"/"+cmd.Name)
		}
	}

	complCmd.Flags = flagsToCompleteFlags(cmd.Flags)
	complCmd.Args = completeCmds[parentPath+"/"+cmd.Name]
	return complCmd
}

// Main function to answer to bash completion calls
func mainComplete() error {
	// Recursively register all commands and subcommands
	// along with global and local flags
	complCmds := make(complete.Commands)
	for _, cmd := range appCmds {
		if cmd.Hidden {
			continue
		}
		complCmds[cmd.Name] = cmdToCompleteCmd(cmd, "")
		for _, alias := range cmd.Aliases {
			complCmds[alias] = cmdToCompleteCmd(cmd, "")
		}
	}
	complFlags := flagsToCompleteFlags(globalFlags)
	mcComplete := complete.Command{
		Sub:         complCmds,
		GlobalFlags: complFlags,
	}
	// Answer to bash completion call
	complete.New(filepath.Base(os.Args[0]), mcComplete).Run()
	return nil
}
