package nacos

import (
	"testing"
)

func TestHeartbeatWorkerRecoversPerTickPanicAndContinues(t *testing.T) {
	heartbeatCalls, registrationCalls := 0, 0
	worker := &nacosHeartbeatWorker{
		maxFails: 1,
		heartbeatFn: func() error {
			heartbeatCalls++
			if heartbeatCalls == 1 {
				panic("injected heartbeat panic")
			}
			return nil
		},
		reregisterFn: func() error {
			registrationCalls++
			return nil
		},
	}
	worker.runHeartbeatCycle(&NacosClient{})
	worker.runHeartbeatCycle(&NacosClient{})
	if heartbeatCalls != 2 {
		t.Fatalf("heartbeat calls=%d, want 2 after recovered panic", heartbeatCalls)
	}
	if registrationCalls != 1 {
		t.Fatalf("re-registration calls=%d, want 1 after failed heartbeat", registrationCalls)
	}
}
