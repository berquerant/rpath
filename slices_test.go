package rpath_test

import (
	"fmt"
	"testing"

	"github.com/berquerant/rpath"
	"github.com/stretchr/testify/assert"
)

func TestFindClosest(t *testing.T) {
	xs := []int{
		1, 2, 4, 5, 7, 10, 12,
	}

	t.Run("Ceiling", func(t *testing.T) {
		for _, tc := range []struct {
			target int
			want   int // -1 means not found
		}{
			{
				target: 13,
				want:   -1,
			},
			{
				target: 1,
				want:   1,
			},
			{
				target: 9,
				want:   5,
			},
			{
				target: 5,
				want:   4,
			},
			{
				target: 3,
				want:   2,
			},
		} {
			t.Run(fmt.Sprintf("%d_%d", tc.target, tc.want), func(t *testing.T) {
				got, found := rpath.FindClosestCeiling(xs, tc.target, func(tgt, elem int) int {
					return tgt - elem
				})
				if tc.want < 0 {
					assert.False(t, found)
					return
				}
				assert.True(t, found)
				assert.Equal(t, tc.want, got)
			})
		}
	})

	t.Run("Floor", func(t *testing.T) {
		for _, tc := range []struct {
			target int
			want   int // -1 means not found
		}{
			{
				target: 13,
				want:   6,
			},
			{
				target: 1,
				want:   -1,
			},
			{
				target: 9,
				want:   4,
			},
			{
				target: 5,
				want:   2,
			},
			{
				target: 3,
				want:   1,
			},
		} {
			t.Run(fmt.Sprintf("%d_%d", tc.target, tc.want), func(t *testing.T) {
				got, found := rpath.FindClosestFloor(xs, tc.target, func(tgt, elem int) int {
					return tgt - elem
				})
				if tc.want < 0 {
					assert.False(t, found)
					return
				}
				assert.True(t, found)
				assert.Equal(t, tc.want, got)
			})
		}
	})
}
