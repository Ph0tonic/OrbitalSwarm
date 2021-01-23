package pathgenerator

import (
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gonum.org/v1/gonum/spatial/r3"
)

type mockDrone struct {
	res []r3.Vec
}

func newMockDrone() *mockDrone {
	return &mockDrone{}
}

func (d *mockDrone) UpdateLocation(location r3.Vec) {
	d.res = append(d.res, location)
}

func TestGenerateBasicPath(t *testing.T) {
	from := []r3.Vec{
		r3.Vec{X: 0, Y: 0, Z: 0},
		r3.Vec{X: 1, Y: 0, Z: 0},
		r3.Vec{X: 0, Y: 0, Z: 1},
		r3.Vec{X: 1, Y: 0, Z: 1},
	}
	dest := []r3.Vec{
		r3.Vec{X: -2, Y: 1, Z: 0}, // 3
		r3.Vec{X: 2, Y: 2, Z: 0},  // 3
		r3.Vec{X: 0, Y: 2, Z: 1},  // 2
		r3.Vec{X: 2, Y: 2, Z: 1},  // 3
	}
	res := generateBasicPath(from, dest)

	expected := [][]r3.Vec{
		[]r3.Vec{
			r3.Vec{X: 0, Y: 1, Z: 0},
			r3.Vec{X: -1, Y: 0, Z: 0},
			r3.Vec{X: -1, Y: 0, Z: 0},
		},
		[]r3.Vec{
			r3.Vec{X: 0, Y: 1, Z: 0},
			r3.Vec{X: 0, Y: 1, Z: 0},
			r3.Vec{X: 1, Y: 0, Z: 0},
		},
		[]r3.Vec{
			r3.Vec{X: 0, Y: 1, Z: 0},
			r3.Vec{X: 0, Y: 1, Z: 0},
			r3.Vec{X: 0, Y: 0, Z: 0},
		},
		[]r3.Vec{
			r3.Vec{X: 0, Y: 1, Z: 0},
			r3.Vec{X: 0, Y: 1, Z: 0},
			r3.Vec{X: 1, Y: 0, Z: 0},
		},
	}

	time.Sleep(time.Second * time.Duration(3))

	log.Println(expected)
	log.Println(res)
	require.Equal(t, len(expected), len(res))
	for i := range expected {
		require.Equal(t, len(expected[i]), len(res[i]))
		for j := range expected[i] {
			require.Equal(t, expected[i][j], res[i][j])
		}
	}
}
