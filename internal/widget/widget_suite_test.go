package widget

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWidget(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Widget Suite")
}
