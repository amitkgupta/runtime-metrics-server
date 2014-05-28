package fake_bbs

import (
	"sync"

	"github.com/cloudfoundry-incubator/runtime-schema/models"
)

type FakeAppManagerBBS struct {
	FileServerGetter

	lrpStartAuctions   []models.LRPStartAuction
	LRPStartAuctionErr error

	desiredLRPs  []models.DesiredLRP
	DesireLRPErr error

	ActualLRPs    []models.LRP
	ActualLRPsErr error

	sync.RWMutex
}

func NewFakeAppManagerBBS() *FakeAppManagerBBS {
	return &FakeAppManagerBBS{}
}

func (fakeBBS *FakeAppManagerBBS) DesireLRP(lrp models.DesiredLRP) error {
	fakeBBS.Lock()
	defer fakeBBS.Unlock()

	fakeBBS.desiredLRPs = append(fakeBBS.desiredLRPs, lrp)
	return fakeBBS.DesireLRPErr
}

func (fakeBBS *FakeAppManagerBBS) DesiredLRPs() []models.DesiredLRP {
	fakeBBS.RLock()
	defer fakeBBS.RUnlock()
	return fakeBBS.desiredLRPs
}

func (fakeBBS *FakeAppManagerBBS) RequestLRPStartAuction(lrp models.LRPStartAuction) error {
	fakeBBS.Lock()
	defer fakeBBS.Unlock()
	fakeBBS.lrpStartAuctions = append(fakeBBS.lrpStartAuctions, lrp)
	return fakeBBS.LRPStartAuctionErr
}

func (fakeBBS *FakeAppManagerBBS) GetLRPStartAuctions() []models.LRPStartAuction {
	fakeBBS.RLock()
	defer fakeBBS.RUnlock()
	return fakeBBS.lrpStartAuctions
}

func (fakeBBS *FakeAppManagerBBS) GetActualLRPsByProcessGuid(string) ([]models.LRP, error) {
	fakeBBS.RLock()
	defer fakeBBS.RUnlock()
	return fakeBBS.ActualLRPs, fakeBBS.ActualLRPsErr
}
