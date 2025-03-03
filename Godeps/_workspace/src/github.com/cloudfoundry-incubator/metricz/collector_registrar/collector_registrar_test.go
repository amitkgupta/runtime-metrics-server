package collector_registrar_test

import (
	"encoding/json"
	"errors"

	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"

	"github.com/cloudfoundry-incubator/metricz"
	. "github.com/cloudfoundry-incubator/metricz/collector_registrar"
)

var _ = Describe("CollectorRegistrar", func() {
	var fakenats *fakeyagnats.FakeYagnats
	var registrar CollectorRegistrar
	var component metricz.Component

	BeforeEach(func() {
		fakenats = fakeyagnats.New()
		registrar = New(fakenats)

		var err error
		component, err = metricz.NewComponent(
			lager.NewLogger("test-component"),
			"Some Component",
			1,
			nil,
			5678,
			[]string{"user", "pass"},
			nil,
		)
		Ω(err).ShouldNot(HaveOccurred())
	})

	It("announces the component to the collector", func() {
		err := registrar.RegisterWithCollector(component)
		Ω(err).ShouldNot(HaveOccurred())

		expected := NewAnnounceComponentMessage(component)

		expectedJson, err := json.Marshal(expected)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(fakenats.PublishedMessages(AnnounceComponentMessageSubject)).Should(ContainElement(
			yagnats.Message{
				Subject: AnnounceComponentMessageSubject,
				Payload: expectedJson,
			},
		))
	})

	Context("when a discover request is received", func() {
		It("responds with the component info", func() {
			err := registrar.RegisterWithCollector(component)
			Ω(err).ShouldNot(HaveOccurred())

			expected := NewAnnounceComponentMessage(component)

			expectedJson, err := json.Marshal(expected)
			Ω(err).ShouldNot(HaveOccurred())

			fakenats.PublishWithReplyTo(
				DiscoverComponentMessageSubject,
				"reply-subject",
				nil,
			)

			Ω(fakenats.PublishedMessages("reply-subject")).Should(ContainElement(
				yagnats.Message{
					Subject: "reply-subject",
					Payload: expectedJson,
				},
			))
		})
	})

	Context("when announcing fails", func() {
		disaster := errors.New("oh no!")

		BeforeEach(func() {
			fakenats.WhenPublishing(AnnounceComponentMessageSubject, func(*yagnats.Message) error {
				return disaster
			})
		})

		It("returns the error", func() {
			err := registrar.RegisterWithCollector(component)
			Ω(err).Should(Equal(disaster))
		})
	})

	Context("when subscribing fails", func() {
		disaster := errors.New("oh no!")

		BeforeEach(func() {
			fakenats.WhenSubscribing(DiscoverComponentMessageSubject, func(yagnats.Callback) error {
				return disaster
			})
		})

		It("returns the error", func() {
			err := registrar.RegisterWithCollector(component)
			Ω(err).Should(Equal(disaster))
		})
	})
})
