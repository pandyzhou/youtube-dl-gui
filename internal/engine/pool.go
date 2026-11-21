package engine

import (
	"context"
	"sync"
)

// pool runs tasks with a changeable limit, taking turns between jobs so
// one long playlist cannot hold every slot while others wait.
type pool struct {
	mu      sync.Mutex
	cond    *sync.Cond
	limit   int
	running int
	queues  []*taskQueue
	closed  bool
}

type taskQueue struct {
	job   string
	tasks []task
}

type task struct {
	ctx context.Context
	run func(context.Context)
}

func newPool(limit int) *pool {
	p := &pool{limit: max(limit, 1)}
	p.cond = sync.NewCond(&p.mu)
	go p.loop()
	return p
}

// submit queues fn for job; it runs with ctx unless ctx ends first.
func (p *pool) submit(job string, ctx context.Context, fn func(context.Context)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, q := range p.queues {
		if q.job == job {
			q.tasks = append(q.tasks, task{ctx, fn})
			p.cond.Broadcast()
			return
		}
	}
	p.queues = append(p.queues, &taskQueue{job: job, tasks: []task{{ctx, fn}}})
	p.cond.Broadcast()
}

// drop forgets a job's queued tasks.
func (p *pool) drop(job string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, q := range p.queues {
		if q.job == job {
			p.queues = append(p.queues[:i], p.queues[i+1:]...)
			return
		}
	}
}

func (p *pool) setLimit(n int) {
	p.mu.Lock()
	p.limit = max(n, 1)
	p.mu.Unlock()
	p.cond.Broadcast()
}

func (p *pool) close() {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	p.cond.Broadcast()
}

func (p *pool) loop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for {
		for !p.closed && (p.running >= p.limit || len(p.queues) == 0) {
			p.cond.Wait()
		}
		if p.closed {
			return
		}
		q := p.queues[0]
		t := q.tasks[0]
		q.tasks = q.tasks[1:]
		p.queues = p.queues[1:]
		if len(q.tasks) > 0 {
			p.queues = append(p.queues, q) // the job's turn ends
		}
		if t.ctx.Err() != nil {
			continue
		}
		p.running++
		go func() {
			defer func() {
				p.mu.Lock()
				p.running--
				p.mu.Unlock()
				p.cond.Broadcast()
			}()
			t.run(t.ctx)
		}()
	}
}
