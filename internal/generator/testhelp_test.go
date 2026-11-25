package generator

import "math/rand"

func newRand() *rand.Rand { return rand.New(rand.NewSource(1)) }
