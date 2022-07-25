// Copyright 2018 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
)

// NewCmdName creates a new cobra.Command for the name subcommand.
func NewCmdName(options *[]crane.Option) *cobra.Command {
	var lock bool
	cmd := &cobra.Command{
		Use:   "name IMAGE",
		Short: "Get the cleaned, fully qualified image reference name",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			img := args[0]
			o := crane.GetOptions(*options...)
			ref, err := name.ParseReference(img, o.Name...)
			if err != nil {
				return err
			}

			if lock {
				dgst, err := crane.Digest(img, *options...)
				if err != nil {
					return err
				}
				// name
				fmt.Println(ref.Context().String() + "@" + dgst)
			} else {
				fmt.Println(ref.Name())
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&lock, "lock", false, "lock the image reference to the digest currently found in the registry")

	return cmd
}
