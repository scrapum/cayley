// Copyright 2014 The Cayley Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package graph

// "Value Comparison" is a unary operator -- a filter across the values in the
// relevant subiterator.
//
// This is hugely useful for things like provenance, but value ranges in general
// come up from time to time. At *worst* we're as big as our underlying iterator.
// At best, we're the null iterator.
//
// This is ripe for backend-side optimization. If you can run a value iterator,
// from a sorted set -- some sort of value index, then go for it.
//
// In MQL terms, this is the [{"age>=": 21}] concept.

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

type ComparisonOperator int

const (
	kCompareLT ComparisonOperator = iota
	kCompareLTE
	kCompareGT
	kCompareGTE
	// Why no Equals? Because that's usually an AndIterator.
)

type ValueComparisonIterator struct {
	BaseIterator
	subIt           Iterator
	op              ComparisonOperator
	comparisonValue interface{}
	ts              TripleStore
}

func NewValueComparisonIterator(
	subIt Iterator,
	operator ComparisonOperator,
	value interface{},
	ts TripleStore) *ValueComparisonIterator {

	var vc ValueComparisonIterator
	BaseIteratorInit(&vc.BaseIterator)
	vc.subIt = subIt
	vc.op = operator
	vc.comparisonValue = value
	vc.ts = ts
	return &vc
}

// Here's the non-boilerplate part of the ValueComparison iterator. Given a value
// and our operator, determine whether or not we meet the requirement.
func (it *ValueComparisonIterator) doComparison(val TSVal) bool {
	//TODO(barakmich): Implement string comparison.
	nodeStr := it.ts.GetNameFor(val)
	switch cVal := it.comparisonValue.(type) {
	case int:
		cInt := int64(cVal)
		intVal, err := strconv.ParseInt(nodeStr, 10, 64)
		if err != nil {
			return false
		}
		return RunIntOp(intVal, it.op, cInt)
	case int64:
		intVal, err := strconv.ParseInt(nodeStr, 10, 64)
		if err != nil {
			return false
		}
		return RunIntOp(intVal, it.op, cVal)
	default:
		return true
	}
}

func (it *ValueComparisonIterator) Close() {
	it.subIt.Close()
}

func RunIntOp(a int64, op ComparisonOperator, b int64) bool {
	switch op {
	case kCompareLT:
		return a < b
	case kCompareLTE:
		return a <= b
	case kCompareGT:
		return a > b
	case kCompareGTE:
		return a >= b
	default:
		log.Fatal("Unknown operator type")
		return false
	}
}

func (it *ValueComparisonIterator) Reset() {
	it.subIt.Reset()
}

func (it *ValueComparisonIterator) Clone() Iterator {
	out := NewValueComparisonIterator(it.subIt.Clone(), it.op, it.comparisonValue, it.ts)
	out.CopyTagsFrom(it)
	return out
}

func (it *ValueComparisonIterator) Next() (TSVal, bool) {
	var val TSVal
	var ok bool
	for {
		val, ok = it.subIt.Next()
		if !ok {
			return nil, false
		}
		if it.doComparison(val) {
			break
		}
	}
	it.Last = val
	return val, ok
}

func (it *ValueComparisonIterator) NextResult() bool {
	for {
		hasNext := it.subIt.NextResult()
		if !hasNext {
			return false
		}
		if it.doComparison(it.subIt.LastResult()) {
			return true
		}
	}
	it.Last = it.subIt.LastResult()
	return true
}

func (it *ValueComparisonIterator) Check(val TSVal) bool {
	if !it.doComparison(val) {
		return false
	}
	return it.subIt.Check(val)
}

// If we failed the check, then the subiterator should not contribute to the result
// set. Otherwise, go ahead and tag it.
func (it *ValueComparisonIterator) TagResults(out *map[string]TSVal) {
	it.BaseIterator.TagResults(out)
	it.subIt.TagResults(out)
}

// Registers the value-comparison iterator.
func (it *ValueComparisonIterator) Type() string { return "value-comparison" }

// Prints the value-comparison and its subiterator.
func (it *ValueComparisonIterator) DebugString(indent int) string {
	return fmt.Sprintf("%s(%s\n%s)",
		strings.Repeat(" ", indent),
		it.Type(), it.subIt.DebugString(indent+4))
}

// There's nothing to optimize, locally, for a value-comparison iterator.
// Replace the underlying iterator if need be.
// potentially replace it.
func (it *ValueComparisonIterator) Optimize() (Iterator, bool) {
	newSub, changed := it.subIt.Optimize()
	if changed {
		it.subIt.Close()
		it.subIt = newSub
	}
	return it, false
}

// We're only as expensive as our subiterator.
// Again, optimized value comparison iterators should do better.
func (it *ValueComparisonIterator) GetStats() *IteratorStats {
	return it.subIt.GetStats()
}
