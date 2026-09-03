package postgres

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPostgresUnit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Postgres Unit Suite")
}
