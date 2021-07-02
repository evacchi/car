// Copyright 2021 Tetrate
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package car

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"

	"github.com/tetratelabs/car/internal"
	"github.com/tetratelabs/car/internal/patternmatcher"
)

// Car is like tar, except for containers.
type Car interface {
	// List prints any files not-filtered from the image layers of the given tag and platform.
	List(ctx context.Context, tag, platform string) error
}

type car struct {
	registry internal.Registry
	out      io.Writer
	// patterns just like tar. Ex "car -tf image:tag foo/* bar.txt"
	patterns             []string
	verbose, veryVerbose bool
}

// New creates a new instance of Car
func New(registry internal.Registry, out io.Writer, patterns []string, verbose, veryVerbose bool) Car {
	return &car{
		registry:    registry,
		out:         out,
		patterns:    patterns,
		verbose:     verbose || veryVerbose,
		veryVerbose: veryVerbose,
	}
}
func (c *car) List(ctx context.Context, tag, platform string) error {
	img, err := c.registry.GetImage(ctx, tag, platform)
	if err != nil {
		return err
	}
	if c.veryVerbose {
		fmt.Fprintln(c.out, img.String()) //nolint
	}

	pm := patternmatcher.New(c.patterns)
	rf := func(name string, size, mode int64, modTime time.Time, _ io.Reader) error {
		if !pm.MatchesPattern(name) {
			return nil
		}
		if c.verbose {
			fmt.Fprintf(c.out, "%s\t%d\t%s\t%s\n", fs.FileMode(mode), size, modTime.Format(time.Stamp), name)
		} else {
			fmt.Fprintln(c.out, name)
		}
		return nil
	}

	for _, layer := range img.FilesystemLayers {
		if c.veryVerbose {
			fmt.Fprintln(c.out, layer.String()) //nolint
		}
		if err := c.registry.ReadFilesystemLayer(ctx, layer, rf); err != nil {
			return err
		}
	}

	unmatched := pm.Unmatched()
	if len(unmatched) > 0 {
		return fmt.Errorf("%s not found in layer", strings.Join(unmatched, ", "))
	}
	return nil
}