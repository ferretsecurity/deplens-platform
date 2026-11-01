package blob

import "context"

type Store interface {
	Put(ctx context.Context, payload []byte) (artifactKey string, sha256 string, err error)
}

type Reader interface {
	Get(ctx context.Context, artifactKey string) ([]byte, error)
}
