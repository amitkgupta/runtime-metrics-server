package lrp_bbs_test

import (
	"time"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry-incubator/runtime-schema/bbs/lrp_bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs/shared"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/cloudfoundry/storeadapter"
	. "github.com/cloudfoundry/storeadapter/storenodematchers"
)

var _ = Describe("Start Auction", func() {
	var bbs *LRPBBS

	BeforeEach(func() {
		bbs = New(etcdClient)
	})

	Describe("RequestLRPStartAuction", func() {
		var auctionLRP models.LRPStartAuction

		BeforeEach(func() {
			auctionLRP = models.LRPStartAuction{
				ProcessGuid: "some-guid",
				Index:       1,
				Actions: []models.ExecutorAction{
					{
						Action: models.RunAction{
							Script: "cat /tmp/file",
							Env: []models.EnvironmentVariable{
								{
									Key:   "PATH",
									Value: "the-path",
								},
							},
							Timeout: time.Second,
						},
					},
				},
			}
		})

		It("creates /v1/start/<guid>/index", func() {
			err := bbs.RequestLRPStartAuction(auctionLRP)
			Ω(err).ShouldNot(HaveOccurred())

			node, err := etcdClient.Get("/v1/start/some-guid/1")
			Ω(err).ShouldNot(HaveOccurred())

			auctionLRP.State = models.LRPStartAuctionStatePending
			Ω(node.Value).Should(Equal(auctionLRP.ToJSON()))
		})

		Context("when the key already exists", func() {
			It("should error", func() {
				err := bbs.RequestLRPStartAuction(auctionLRP)
				Ω(err).ShouldNot(HaveOccurred())

				err = bbs.RequestLRPStartAuction(auctionLRP)
				Ω(err).Should(MatchError(storeadapter.ErrorKeyExists))
			})
		})

		Context("when the store is out of commission", func() {
			itRetriesUntilStoreComesBack(func() error {
				return bbs.RequestLRPStartAuction(auctionLRP)
			})
		})
	})

	Describe("WatchForLRPStartAuction", func() {
		var (
			events     <-chan models.LRPStartAuction
			stop       chan<- bool
			errors     <-chan error
			stopped    bool
			auctionLRP models.LRPStartAuction
		)

		BeforeEach(func() {
			auctionLRP = models.LRPStartAuction{
				ProcessGuid: "some-guid",
				Index:       1,
				Actions: []models.ExecutorAction{
					{
						Action: models.RunAction{
							Script: "cat /tmp/file",
							Env: []models.EnvironmentVariable{
								{
									Key:   "PATH",
									Value: "the-path",
								},
							},
							Timeout: time.Second,
						},
					},
				},
			}
			events, stop, errors = bbs.WatchForLRPStartAuction()
		})

		AfterEach(func() {
			if !stopped {
				stop <- true
			}
		})

		It("sends an event down the pipe for creates", func() {
			err := bbs.RequestLRPStartAuction(auctionLRP)
			Ω(err).ShouldNot(HaveOccurred())

			auctionLRP.State = models.LRPStartAuctionStatePending
			Eventually(events).Should(Receive(Equal(auctionLRP)))
		})

		It("sends an event down the pipe for updates", func() {
			err := bbs.RequestLRPStartAuction(auctionLRP)
			Ω(err).ShouldNot(HaveOccurred())

			auctionLRP.State = models.LRPStartAuctionStatePending
			Eventually(events).Should(Receive(Equal(auctionLRP)))

			err = etcdClient.SetMulti([]storeadapter.StoreNode{
				{
					Key:   shared.LRPStartAuctionSchemaPath(auctionLRP),
					Value: auctionLRP.ToJSON(),
				},
			})
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(events).Should(Receive(Equal(auctionLRP)))
		})

		It("does not send an event down the pipe for deletes", func() {
			err := bbs.RequestLRPStartAuction(auctionLRP)
			Ω(err).ShouldNot(HaveOccurred())

			auctionLRP.State = models.LRPStartAuctionStatePending
			Eventually(events).Should(Receive(Equal(auctionLRP)))

			err = bbs.ResolveLRPStartAuction(auctionLRP)
			Ω(err).ShouldNot(HaveOccurred())

			Consistently(events).ShouldNot(Receive())
		})

		It("closes the events and errors channel when told to stop", func() {
			stop <- true
			stopped = true

			err := bbs.RequestLRPStartAuction(auctionLRP)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(events).Should(BeClosed())
			Ω(errors).Should(BeClosed())
		})
	})

	Describe("ClaimLRPStartAuction", func() {
		var auctionLRP models.LRPStartAuction

		BeforeEach(func() {
			auctionLRP = models.LRPStartAuction{
				ProcessGuid: "some-guid",
				Index:       1,
				Actions: []models.ExecutorAction{
					{
						Action: models.RunAction{
							Script: "cat /tmp/file",
							Env: []models.EnvironmentVariable{
								{
									Key:   "PATH",
									Value: "the-path",
								},
							},
							Timeout: time.Second,
						},
					},
				},
			}

			err := bbs.RequestLRPStartAuction(auctionLRP)

			auctionLRP.State = models.LRPStartAuctionStatePending
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when claiming a requested LRP auction", func() {
			It("sets the state to claimed", func() {
				err := bbs.ClaimLRPStartAuction(auctionLRP)
				Ω(err).ShouldNot(HaveOccurred())

				expectedAuctionLRP := auctionLRP
				expectedAuctionLRP.State = models.LRPStartAuctionStateClaimed

				node, err := etcdClient.Get("/v1/start/some-guid/1")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(node).Should(MatchStoreNode(storeadapter.StoreNode{
					Key:   "/v1/start/some-guid/1",
					Value: expectedAuctionLRP.ToJSON(),
				}))
			})

			Context("when the store is out of commission", func() {
				itRetriesUntilStoreComesBack(func() error {
					return bbs.ClaimLRPStartAuction(auctionLRP)
				})
			})
		})

		Context("When claiming an LRP auction that is not in the pending state", func() {
			BeforeEach(func() {
				err := bbs.ClaimLRPStartAuction(auctionLRP)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("returns an error", func() {
				err := bbs.ClaimLRPStartAuction(auctionLRP)
				Ω(err).Should(HaveOccurred())
			})
		})
	})

	Describe("ResolveLRPStartAuction", func() {
		var auctionLRP models.LRPStartAuction

		BeforeEach(func() {
			auctionLRP = models.LRPStartAuction{
				ProcessGuid: "some-guid",
				Index:       1,
				Actions: []models.ExecutorAction{
					{
						Action: models.RunAction{
							Script: "cat /tmp/file",
							Env: []models.EnvironmentVariable{
								{
									Key:   "PATH",
									Value: "the-path",
								},
							},
							Timeout: time.Second,
						},
					},
				},
			}

			err := bbs.RequestLRPStartAuction(auctionLRP)

			auctionLRP.State = models.LRPStartAuctionStatePending
			Ω(err).ShouldNot(HaveOccurred())

			err = bbs.ClaimLRPStartAuction(auctionLRP)
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("should remove /v1/start/<guid>/<index>", func() {
			err := bbs.ResolveLRPStartAuction(auctionLRP)
			Ω(err).ShouldNot(HaveOccurred())

			_, err = etcdClient.Get("/v1/start/some-guid/1")
			Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))
		})

		Context("when the store is out of commission", func() {
			itRetriesUntilStoreComesBack(func() error {
				err := bbs.ResolveLRPStartAuction(auctionLRP)
				return err
			})
		})
	})
})
