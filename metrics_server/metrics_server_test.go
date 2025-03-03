package metrics_server_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"syscall"

	"github.com/cloudfoundry-incubator/cf-lager"
	"github.com/cloudfoundry-incubator/metricz/instrumentation"
	"github.com/cloudfoundry-incubator/metricz/localip"
	. "github.com/cloudfoundry-incubator/runtime-metrics-server/metrics_server"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs/fake_bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Metrics Server", func() {
	var (
		fakenats   *fakeyagnats.FakeYagnats
		logger     lager.Logger
		bbs        *fake_bbs.FakeMetricsBBS
		port       uint32
		server     *MetricsServer
		httpClient *http.Client
	)

	BeforeEach(func() {
		fakenats = fakeyagnats.New()
		bbs = fake_bbs.NewFakeMetricsBBS()
		logger = cf_lager.New("fake-logger")

		port = 34567 + uint32(config.GinkgoConfig.ParallelNode)

		server = New(fakenats, bbs, logger, Config{
			Port:     port,
			Username: "the-username",
			Password: "the-password",
			Index:    3,
		})

		httpClient = &http.Client{
			Transport: &http.Transport{},
		}
	})

	Describe("Envoke", func() {
		var (
			payloadChan chan []byte
			myIP        string
			process     ifrit.Process
		)

		BeforeEach(func() {
			var err error
			myIP, err = localip.LocalIP()
			Ω(err).ShouldNot(HaveOccurred())

			payloadChan = make(chan []byte, 1)
			fakenats.Subscribe("vcap.component.announce", func(msg *yagnats.Message) {
				payloadChan <- msg.Payload
			})

			process = ifrit.Envoke(server)
		})

		AfterEach(func(done Done) {
			process.Signal(syscall.SIGTERM)
			<-process.Wait()
			close(done)
		})

		It("announces to the collector with the right type, port, credentials and index", func(done Done) {
			payload := <-payloadChan
			response := make(map[string]interface{})
			json.Unmarshal(payload, &response)

			Ω(response["type"]).Should(Equal("runtime"))

			Ω(strings.HasSuffix(response["host"].(string), fmt.Sprintf(":%d", port))).Should(BeTrue())

			Ω(response["credentials"]).Should(Equal([]interface{}{
				"the-username",
				"the-password",
			}))

			Ω(response["index"]).Should(Equal(float64(3)))

			close(done)
		}, 3)

		Describe("the varz endpoint", func() {
			var varzMessage instrumentation.VarzMessage

			JustBeforeEach(func() {
				request, _ := http.NewRequest("GET", fmt.Sprintf("http://%s:%d/varz", myIP, port), nil)
				request.SetBasicAuth("the-username", "the-password")

				response, err := httpClient.Do(request)
				Ω(err).ShouldNot(HaveOccurred())
				bytes, _ := ioutil.ReadAll(response.Body)

				varzMessage = instrumentation.VarzMessage{}

				err = json.Unmarshal(bytes, &varzMessage)
				Ω(err).ShouldNot(HaveOccurred())
			})

			Context("when the read from the store succeeds", func() {
				BeforeEach(func() {
					bbs.GetAllTasksReturns.Models = []models.Task{
						models.Task{State: models.TaskStatePending},
						models.Task{State: models.TaskStatePending},
						models.Task{State: models.TaskStatePending},

						models.Task{State: models.TaskStateClaimed},
						models.Task{State: models.TaskStateClaimed},

						models.Task{State: models.TaskStateRunning},

						models.Task{State: models.TaskStateCompleted},
						models.Task{State: models.TaskStateCompleted},
						models.Task{State: models.TaskStateCompleted},
						models.Task{State: models.TaskStateCompleted},

						models.Task{State: models.TaskStateResolving},
						models.Task{State: models.TaskStateResolving},
					}

					bbs.GetServiceRegistrationsReturns.Registrations = models.ServiceRegistrations{
						{Name: models.ExecutorServiceName, Id: "purple-elephants"},
					}
				})

				It("reports the correct name", func() {
					Ω(varzMessage.Name).Should(Equal("runtime"))
				})

				It("returns the number of tasks in each state", func() {
					Ω(varzMessage.Contexts[0]).Should(Equal(instrumentation.Context{
						Name: "Tasks",
						Metrics: []instrumentation.Metric{
							{
								Name:  "Pending",
								Value: float64(3),
							},
							{
								Name:  "Claimed",
								Value: float64(2),
							},
							{
								Name:  "Running",
								Value: float64(1),
							},
							{
								Name:  "Completed",
								Value: float64(4),
							},
							{
								Name:  "Resolving",
								Value: float64(2),
							},
						},
					}))
				})

				It("returns the number of registered services by service type", func() {
					Ω(varzMessage.Contexts[1]).Should(Equal(instrumentation.Context{
						Name: "ServiceRegistrations",
						Metrics: []instrumentation.Metric{
							{Name: "Executor", Value: float64(1)},
							{Name: "FileServer", Value: float64(0)},
						},
					}))
				})
			})

			Context("when there is an error reading from the store", func() {
				BeforeEach(func() {
					bbs.GetAllTasksReturns.Err = errors.New("Doesn't work")
				})

				It("reports -1 for all of the task counts", func() {
					Ω(varzMessage.Contexts[0]).Should(Equal(instrumentation.Context{
						Name: "Tasks",
						Metrics: []instrumentation.Metric{
							{
								Name:  "Pending",
								Value: float64(-1),
							},
							{
								Name:  "Claimed",
								Value: float64(-1),
							},
							{
								Name:  "Running",
								Value: float64(-1),
							},
							{
								Name:  "Completed",
								Value: float64(-1),
							},
							{
								Name:  "Resolving",
								Value: float64(-1),
							},
						},
					}))
				})
			})
		})

		Describe("the healthz endpoint", func() {
			It("returns success", func() {
				request, _ := http.NewRequest("GET", fmt.Sprintf("http://%s:%d/healthz", myIP, port), nil)
				request.SetBasicAuth("the-username", "the-password")

				response, err := httpClient.Do(request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response.StatusCode).To(Equal(200))

				body, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(string(body)).Should(Equal("ok"))
			})
		})
	})
})
