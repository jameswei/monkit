// Copyright (C) 2016 Space Monkey, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package monkit

import (
	"sort"
)

// IntDist keeps statistics about values such as
// low/high/recent/average/quantiles. Not threadsafe. Construct with
// NewIntDist(). Fields are expected to be read from but not written to.
type IntDist struct {
	// Low and High are the lowest and highest values observed since
	// construction or the last reset.
	Low, High int64

	// Recent is the last observed value.
	Recent int64

	// Count is the number of observed values since construction or the last
	// reset.
	Count int64

	// Sum is the sum of all the observed values since construction or the last
	// reset.
	Sum int64

	reservoir [reservoirSize]float32
	lcg       lcg
	sorted    bool
}

func initIntDist(v *IntDist) {
	v.lcg = newLCG()
}

// NewIntDist creates a distribution of int64s.
func NewIntDist() (d *IntDist) {
	d = &IntDist{}
	initIntDist(d)
	return d
}

// Insert adds a value to the distribution, updating appropriate values.
func (d *IntDist) Insert(val int64) {
	if d.Count != 0 {
		if val < d.Low {
			d.Low = val
		}
		if val > d.High {
			d.High = val
		}
	} else {
		d.Low = val
		d.High = val
	}
	d.Recent = val
	d.Sum += val

	index := d.Count
	d.Count += 1

	if index < reservoirSize {
		d.reservoir[index] = float32(val)
		d.sorted = false
	} else {
		// fast, but kind of biased. probably okay
		j := d.lcg.Uint64() % uint64(d.Count)
		if j < reservoirSize {
			d.reservoir[int(j)] = float32(val)
			d.sorted = false
		}
	}
}

// Average calculates and returns the average of inserted values.
func (d *IntDist) Average() int64 {
	if d.Count > 0 {
		return d.Sum / int64(d.Count)
	}
	return 0
}

// Query will return the approximate value at the given quantile, where
// 0 <= quantile <= 1.
func (d *IntDist) Query(quantile float64) int64 {
	if quantile <= 0 {
		return d.Low
	}
	if quantile >= 1 {
		return d.High
	}

	rlen := int(reservoirSize)
	if int64(rlen) > d.Count {
		rlen = int(d.Count)
	}

	if rlen < 2 {
		return int64(d.reservoir[0])
	}

	idx_float := quantile * float64(rlen-1)
	idx := int(idx_float)

	reservoir := d.reservoir[:rlen]
	if !d.sorted {
		sort.Sort(float32Slice(reservoir))
		d.sorted = true
	}
	diff := idx_float - float64(idx)
	prior := float64(reservoir[idx])
	return int64(prior + diff*(float64(reservoir[idx+1])-prior))
}

// Copy returns a full copy of the entire distribution.
func (d *IntDist) Copy() *IntDist {
	cp := *d
	cp.lcg = newLCG()
	return &cp
}

func (d *IntDist) Reset() {
	d.Low, d.High, d.Recent, d.Count, d.Sum = 0, 0, 0, 0, 0
	// resetting count will reset the quantile reservoir
}
