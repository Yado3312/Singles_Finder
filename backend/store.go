package main

import (
	"context"
)

type requestStore interface {
	mapRequestStore
	Ping(context.Context) error
}
