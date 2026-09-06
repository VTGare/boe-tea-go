//go:build integration

package mongo

import (
	"context"
	"os"
	"testing"

	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/boe-tea-go/store/conformance"
	"go.mongodb.org/mongo-driver/v2/bson"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMongoIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mongo Integration Suite")
}

var testStore store.Store

func init() {
	conformance.Specs(func() store.Store { return testStore })
}

func mongoURI() string {
	if uri := os.Getenv("MONGO_URI"); uri != "" {
		return uri
	}

	return "mongodb://127.0.0.1:27018"
}

var _ = BeforeSuite(func() {
	var err error

	testStore, err = New(context.Background(), mongoURI(), "boetea_test")
	Expect(err).NotTo(HaveOccurred())
	Expect(testStore.Init(context.Background())).To(Succeed())
})

var _ = AfterSuite(func() {
	Expect(testStore.Close(context.Background())).To(Succeed())
})

var _ = BeforeEach(func() {
	ms, ok := testStore.(*mongoStore)
	Expect(ok).To(BeTrue())

	for _, col := range []string{"artworks", "bookmarks", "counters", "guilds", "users"} {
		_, err := ms.database.Collection(col).DeleteMany(context.Background(), bson.M{})
		Expect(err).NotTo(HaveOccurred())
	}
})
