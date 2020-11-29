package client

import (
	"github.com/buraksezer/consistent"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Client Suite")
}

var _ = Describe("ConsistentHashRing", func() {
	It("Should return the same proxy for the given key.", func() {
		cfg := consistent.Config{
			PartitionCount:    271,
			ReplicationFactor: 20,
			Load:              1.25,
			Hasher:            hasher{},
		}
		members := make([]consistent.Member, 2)

		members[0] = clientMember("10.0.109.88:6378")
		members[1] = clientMember("10.0.109.89:6378")

		ring := consistent.New(members, cfg)

		key := "mr.srt-res-0"

		//member1 := c.Ring.LocateKey([]byte(key))
		//host1 := member1.String()

		//member2 := c.Ring.LocateKey([]byte(key))
		//host2 := member2.String()

		for i := 0; i < 10; i++ {
			Expect(ring.LocateKey([]byte(key)).String()).To(Equal("10.0.109.89:6378"))
		}
	})
})
