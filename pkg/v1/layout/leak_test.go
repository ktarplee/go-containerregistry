package layout

import (
	"math/rand"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/random"

	"github.com/fortytw2/leaktest"
)

func TestLeak(t *testing.T) {
	defer leaktest.Check(t)()

	p, err := Write(t.TempDir(), empty.Index)
	if err != nil {
		t.Fatal(err)
	}

	// Randomly generate an image
	img1, err := random.Image(10, 1, random.WithSource(rand.NewSource(1)))
	if err != nil {
		t.Fatal(err)
	}

	// FIXME commenting this out either of these Append functions prevents the goroutine leak
	// So using img1 twice seems to be the issue
	err = p.AppendImage(img1)
	if err != nil {
		t.Fatal(err)
	}

	err = p.AppendImage(img1)
	if err != nil {
		t.Fatal(err)
	}

	// idx2 := mutate.AppendManifests(empty.Index,
	// 	mutate.IndexAddendum{Add: img1},
	// )
	// err = p.AppendIndex(idx2)
	// if err != nil {
	// 	t.Fatal(err)
	// }
}
