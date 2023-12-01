/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2017-2023 WireGuard LLC. All Rights Reserved.
 */

package device

import (
	"sync"
	"sync/atomic"
)

type WaitPool struct {
	pool  []any
	cond  sync.Cond
	lock  sync.Mutex
	count atomic.Uint32
	max   uint32
}

var (
	inboundElementsContainer  *WaitPool
	outboundElementsContainer *WaitPool
	messageBuffers            *WaitPool
	inboundElements           *WaitPool
	outboundElements          *WaitPool
)

func NewWaitPool(max uint32, new func() any) *WaitPool {
	pool := make([]any, max)
	var i uint32
	for i = 0; i < max; i++ {
		pool[i] = new()
	}
	p := &WaitPool{pool: pool, max: max}
	p.cond = sync.Cond{L: &p.lock}
	return p
}

func (p *WaitPool) Get() any {

	for p.count.Load() >= p.max {
		p.cond.Wait()
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	item := p.pool[p.count.Load()]
	p.count.Add(1)
	return item
}

func (p *WaitPool) Put(x any) {
	p.lock.Lock()
	p.count.Add(^uint32(0))
	p.pool[p.count.Load()] = x
	p.lock.Unlock()

	p.cond.Signal()
}

func (device *Device) PopulatePools() {
	if inboundElementsContainer == nil {
		inboundElementsContainer = NewWaitPool(PreallocatedBuffersPerPool, func() any {
			s := make([]*QueueInboundElement, 0, device.BatchSize())
			return &QueueInboundElementsContainer{elems: s}
		})
	}
	device.pool.inboundElementsContainer = inboundElementsContainer

	if outboundElementsContainer == nil {
		outboundElementsContainer = NewWaitPool(PreallocatedBuffersPerPool, func() any {
			s := make([]*QueueOutboundElement, 0, device.BatchSize())
			return &QueueOutboundElementsContainer{elems: s}
		})
	}
	device.pool.outboundElementsContainer = outboundElementsContainer

	if messageBuffers == nil {
		messageBuffers = NewWaitPool(PreallocatedBuffersPerPool, func() any {
			return new([MaxMessageSize]byte)
		})
	}
	device.pool.messageBuffers = messageBuffers

	if inboundElements == nil {
		inboundElements = NewWaitPool(PreallocatedBuffersPerPool, func() any {
			return new(QueueInboundElement)
		})
	}
	device.pool.inboundElements = inboundElements

	if outboundElements == nil {
		outboundElements = NewWaitPool(PreallocatedBuffersPerPool, func() any {
			return new(QueueOutboundElement)
		})
	}
	device.pool.outboundElements = outboundElements
}

func (device *Device) GetInboundElementsContainer() *QueueInboundElementsContainer {
	c := device.pool.inboundElementsContainer.Get().(*QueueInboundElementsContainer)
	c.Mutex = sync.Mutex{}
	return c
}

func (device *Device) PutInboundElementsContainer(c *QueueInboundElementsContainer) {
	for i := range c.elems {
		c.elems[i] = nil
	}
	c.elems = c.elems[:0]
	device.pool.inboundElementsContainer.Put(c)
}

func (device *Device) GetOutboundElementsContainer() *QueueOutboundElementsContainer {
	c := device.pool.outboundElementsContainer.Get().(*QueueOutboundElementsContainer)
	c.Mutex = sync.Mutex{}
	return c
}

func (device *Device) PutOutboundElementsContainer(c *QueueOutboundElementsContainer) {
	for i := range c.elems {
		c.elems[i] = nil
	}
	c.elems = c.elems[:0]
	device.pool.outboundElementsContainer.Put(c)
}

func (device *Device) GetMessageBuffer() *[MaxMessageSize]byte {
	return device.pool.messageBuffers.Get().(*[MaxMessageSize]byte)
}

func (device *Device) PutMessageBuffer(msg *[MaxMessageSize]byte) {
	device.pool.messageBuffers.Put(msg)
}

func (device *Device) GetInboundElement() *QueueInboundElement {
	return device.pool.inboundElements.Get().(*QueueInboundElement)
}

func (device *Device) PutInboundElement(elem *QueueInboundElement) {
	elem.clearPointers()
	device.pool.inboundElements.Put(elem)
}

func (device *Device) GetOutboundElement() *QueueOutboundElement {
	return device.pool.outboundElements.Get().(*QueueOutboundElement)
}

func (device *Device) PutOutboundElement(elem *QueueOutboundElement) {
	elem.clearPointers()
	device.pool.outboundElements.Put(elem)
}
