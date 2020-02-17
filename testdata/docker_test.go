package testdata

import (
	"testing"

	"github.com/XyuWang/dockertest"
)

func TestStart(t *testing.T) {
	path := "./docker-compose.yml"
	dockertest.Run(path)
}
