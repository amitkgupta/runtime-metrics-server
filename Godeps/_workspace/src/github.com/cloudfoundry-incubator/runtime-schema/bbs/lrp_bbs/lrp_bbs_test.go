package lrp_bbs_test

import (
	. "github.com/cloudfoundry-incubator/runtime-schema/bbs/lrp_bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/cloudfoundry/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LRP", func() {
	var bbs *LRPBBS

	BeforeEach(func() {
		bbs = New(etcdClient)
	})

	Describe("DesireLRP", func() {
		var lrp models.DesiredLRP

		BeforeEach(func() {
			lrp = models.DesiredLRP{
				ProcessGuid: "some-process-guid",
				Instances:   5,
				Stack:       "some-stack",
				MemoryMB:    1024,
				DiskMB:      512,
				Routes:      []string{"route-1", "route-2"},
			}
		})

		It("creates /v1/desired/<process-guid>/<index>", func() {
			err := bbs.DesireLRP(lrp)
			Ω(err).ShouldNot(HaveOccurred())

			node, err := etcdClient.Get("/v1/desired/some-process-guid")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(node.Value).Should(Equal(lrp.ToJSON()))
		})

		Context("when the store is out of commission", func() {
			itRetriesUntilStoreComesBack(func() error {
				return bbs.DesireLRP(lrp)
			})
		})
	})

	Describe("Adding and removing actual LRPs", func() {
		var lrp models.LRP

		BeforeEach(func() {
			lrp = models.LRP{
				ProcessGuid:  "some-process-guid",
				InstanceGuid: "some-instance-guid",
				Index:        1,

				Host: "1.2.3.4",
				Ports: []models.PortMapping{
					{ContainerPort: 8080, HostPort: 65100},
					{ContainerPort: 8081, HostPort: 65101},
				},
			}
		})

		Describe("ReportActualLRPAsStarting", func() {
			It("creates /v1/actual/<process-guid>/<index>/<instance-guid>", func() {
				err := bbs.ReportActualLRPAsStarting(lrp)
				Ω(err).ShouldNot(HaveOccurred())

				node, err := etcdClient.Get("/v1/actual/some-process-guid/1/some-instance-guid")
				Ω(err).ShouldNot(HaveOccurred())

				expectedLRP := lrp
				expectedLRP.State = models.LRPStateStarting
				Ω(node.Value).Should(MatchJSON(expectedLRP.ToJSON()))
			})

			Context("when the store is out of commission", func() {
				itRetriesUntilStoreComesBack(func() error {
					return bbs.ReportActualLRPAsStarting(lrp)
				})
			})
		})

		Describe("ReportActualLRPAsRunning", func() {
			It("creates /v1/actual/<process-guid>/<index>/<instance-guid>", func() {
				err := bbs.ReportActualLRPAsRunning(lrp)
				Ω(err).ShouldNot(HaveOccurred())

				node, err := etcdClient.Get("/v1/actual/some-process-guid/1/some-instance-guid")
				Ω(err).ShouldNot(HaveOccurred())

				expectedLRP := lrp
				expectedLRP.State = models.LRPStateRunning
				Ω(node.Value).Should(MatchJSON(expectedLRP.ToJSON()))
			})

			Context("when the store is out of commission", func() {
				itRetriesUntilStoreComesBack(func() error {
					return bbs.ReportActualLRPAsRunning(lrp)
				})
			})
		})

		Describe("RemoveActualLRP", func() {
			BeforeEach(func() {
				bbs.ReportActualLRPAsStarting(lrp)
			})

			It("should remove the LRP", func() {
				err := bbs.RemoveActualLRP(lrp)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = etcdClient.Get("/v1/actual/some-process-guid/1/some-instance-guid")
				Ω(err).Should(MatchError(storeadapter.ErrorKeyNotFound))
			})

			Context("when the store is out of commission", func() {
				itRetriesUntilStoreComesBack(func() error {
					return bbs.RemoveActualLRP(lrp)
				})
			})
		})
	})

})
