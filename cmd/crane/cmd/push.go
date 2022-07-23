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
	"io/ioutil"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// NewCmdPush creates a new cobra.Command for the push subcommand.
func NewCmdPush(options *[]crane.Option) *cobra.Command {
	index := false
	imageRefs := ""
	im := indexMatchers{}

	cmd := &cobra.Command{
		Use:   "push PATH IMAGE",
		Short: "Push local image contents to a remote registry",
		Long: `If the PATH is a directory, it will be read as an OCI image layout. Otherwise, PATH is assumed to be a docker-style tarball.
		
		All match options must apply for an image to be included.`,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			path, tag := args[0], args[1]

			o := crane.GetOptions(*options...)
			ref, err := name.ParseReference(tag, o.Name...)
			if err != nil {
				return err
			}

			matchers, err := im.GetMatchers()
			if err != nil {
				return err
			}

			// If the destination contains a digest then we should only push the manifest with that digest
			if digestRef, ok := ref.(name.Digest); ok {
				// add a selector to only select the given digest
				// convert the string to a v1.Hash
				hashRef, err := v1.NewHash(digestRef.DigestStr())
				if err != nil {
					return err
				}
				matchers = append(matchers, match.Digests(hashRef))
			}

			img, err := loadImage(path, index, matchers)
			if err != nil {
				return err
			}

			var h v1.Hash
			switch t := img.(type) {
			case v1.Image:
				if err := remote.Write(ref, t, o.Remote...); err != nil {
					return err
				}
				if h, err = t.Digest(); err != nil {
					return err
				}
			case v1.ImageIndex:
				if err := remote.WriteIndex(ref, t, o.Remote...); err != nil {
					return err
				}
				if h, err = t.Digest(); err != nil {
					return err
				}
			default:
				return fmt.Errorf("cannot push type (%T) to registry", img)
			}

			digest := ref.Context().Digest(h.String())
			if imageRefs != "" {
				return ioutil.WriteFile(imageRefs, []byte(digest.String()), 0600)
			}
			// TODO(mattmoor): think about printing the digest to standard out
			// to facilitate command composition similar to ko build.

			return nil
		},
	}

	cmd.Flags().BoolVar(&index, "index", false, "push a collection of images as a single index, currently required if PATH contains multiple images")
	cmd.Flags().StringVar(&imageRefs, "image-refs", "", "path to file where a list of the published image references will be written")
	cmd.Flags().AddFlagSet(im.GetFlagSet())
	return cmd
}

func loadImage(path string, index bool, matchers []match.Matcher) (partial.WithRawManifest, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !stat.IsDir() {
		img, err := crane.Load(path)
		if err != nil {
			return nil, fmt.Errorf("loading %s as tarball: %w", path, err)
		}
		return img, nil
	}

	l, err := layout.ImageIndexFromPath(path)
	if err != nil {
		return nil, fmt.Errorf("loading %s as OCI layout: %w", path, err)
	}

	// apply selectors to filter images
	m, err := l.IndexManifest()
	if err != nil {
		return nil, err
	}

	filteredManifests := make([]v1.Descriptor, 0, len(m.Manifests))
	for _, d := range m.Manifests {
		if matchesAll(d, matchers) {
			filteredManifests = append(filteredManifests, d)
		}
	}
	m.Manifests = filteredManifests
	// TODO serialize m and set it to l
	l, err = layout.ImageIndexFromPathWithIndex(path, *m)
	if err != nil {
		return nil, err
	}

	if index {
		return l, nil
	}

	if len(m.Manifests) != 1 {
		return nil, fmt.Errorf("layout contains %d entries, consider --index", len(m.Manifests))
	}

	desc := m.Manifests[0]
	if desc.MediaType.IsImage() {
		return l.Image(desc.Digest)
	} else if desc.MediaType.IsIndex() {
		return l.ImageIndex(desc.Digest)
	}

	return nil, fmt.Errorf("layout contains non-image (mediaType: %q), consider --index", desc.MediaType)
}

type indexMatchers struct {
	name        string
	digest      string
	annotations []string
	// platform
	// mediaType
}

func (im *indexMatchers) GetFlagSet() *pflag.FlagSet {
	flags := &pflag.FlagSet{}
	flags.StringVar(&im.digest, "match-digest", "", `digest of image to select.  Only applicable to OCI format.`)
	flags.StringVar(&im.name, "match-name", "", `name of image to select (the one with "org.opencontainers.image.ref.name" annotation that matches).  Only applicable to OCI format.`)
	flags.StringArrayVar(&im.annotations, "match-annotation", nil, `selectors to use to filter the image list based on annotations.  Only applicable to OCI format.
	To filter by original image name use "original=busybox".`)
	return flags
}

func (im *indexMatchers) GetMatchers() ([]match.Matcher, error) {
	matchers := make([]match.Matcher, 0, len(im.annotations))
	if im.digest != "" {
		// convert the string to a v1.Hash
		hashRef, err := v1.NewHash(im.digest)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, match.Digests(hashRef))
	}

	if im.name != "" {
		matchers = append(matchers, match.Name(im.name))
	}

	// convert matchAnnotations into matchers
	for _, s := range im.annotations {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf(`match-annotation "%s" is missing a "=" sign`, s)
		}
		matchers = append(matchers, match.Annotation(parts[0], parts[1]))
	}
	return matchers, nil
}

func matchesAll(desc v1.Descriptor, matchers []match.Matcher) bool {
	for _, matcher := range matchers {
		if !matcher(desc) {
			return false
		}
	}
	return true
}
