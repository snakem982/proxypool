package spider

import (
	"github.com/panjf2000/ants/v2"
	"sync"
	"time"
)

const (
	defaultJobQueueLength = 512 // 默认任务队列长度
)

type Job func(chan struct{})

type TimeoutPool struct {
	antPool *ants.Pool
	wg      sync.WaitGroup
}

// NewTimeoutPoolWithDefaults 初始化一个任务队列长度512
func NewTimeoutPoolWithDefaults() *TimeoutPool {
	p, _ := ants.NewPool(defaultJobQueueLength, func(opts *ants.Options) {
		opts.PreAlloc = true
	})
	return &TimeoutPool{p, sync.WaitGroup{}}
}

// NewTimeoutPool 初始化一个任务队列长度为size
func NewTimeoutPool(size int) *TimeoutPool {
	p, _ := ants.NewPool(size, func(opts *ants.Options) {
		opts.PreAlloc = true
	})
	return &TimeoutPool{p, sync.WaitGroup{}}
}

// SubmitWithTimeout 提交一个任务到协程池
func (p *TimeoutPool) SubmitWithTimeout(job Job, timeout time.Duration) {
	_ = p.antPool.Submit(func() {
		done := make(chan struct{}, 1)
		go job(done)
		select {
		case <-done:
		case <-time.After(timeout):
		}
		p.wg.Done()
	})
}

// StartAndWait 启动并等待协程池内的运行全部运行结束
func (p *TimeoutPool) StartAndWait() {
	p.wg.Wait()
	p.antPool.Release()
}

func (p *TimeoutPool) WaitCount(count int) {
	p.wg.Add(count)
}
