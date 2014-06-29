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

// Defines one of the base iterators, the HasA iterator. The HasA takes a
// subiterator of links, and acts as an iterator of nodes in the given
// direction. The name comes from the idea that a "link HasA subject" or a "link
// HasA predicate".
//
// HasA is weird in that it may return the same value twice if on the Next()
// path. That's okay -- in reality, it can be viewed as returning the value for
// a new triple, but to make logic much simpler, here we have the HasA.
//
// Likewise, it's important to think about Check()ing a HasA. When given a
// value to check, it means "Check all predicates that have this value for your
// direction against the subiterator." This would imply that there's more than
// one possibility for the same Check()ed value. While we could return the
// number of options, it's simpler to return one, and then call NextResult()
// enough times to enumerate the options. (In fact, one could argue that the
// raison d'etre for NextResult() is this iterator).
//
// Alternatively, can be seen as the dual of the LinksTo iterator.

import (
	"container/list"
	"fmt"
	"strings"

	"github.com/barakmich/glog"
)

// A HasaIterator consists of a reference back to the TripleStore that it references,
// a primary subiterator, a direction in which the triples for that subiterator point,
// and a temporary holder for the iterator generated on Check().
type HasaIterator struct {
	BaseIterator
	ts        TripleStore
	primaryIt Iterator
	direction string
	resultIt  Iterator
}

// Construct a new HasA iterator, given the triple subiterator, and the triple
// direction for which it stands.
func NewHasaIterator(ts TripleStore, subIt Iterator, dir string) *HasaIterator {
	var hasa HasaIterator
	BaseIteratorInit(&hasa.BaseIterator)
	hasa.ts = ts
	hasa.primaryIt = subIt
	hasa.direction = dir
	return &hasa
}

// Return our sole subiterator, in a list.List.
func (it *HasaIterator) GetSubIterators() *list.List {
	l := list.New()
	l.PushBack(it.primaryIt)
	return l
}

func (it *HasaIterator) Reset() {
	it.primaryIt.Reset()
	if it.resultIt != nil {
		it.resultIt.Close()
	}
}

func (it *HasaIterator) Clone() Iterator {
	out := NewHasaIterator(it.ts, it.primaryIt.Clone(), it.direction)
	out.CopyTagsFrom(it)
	return out
}

// Direction accessor.
func (it *HasaIterator) Direction() string { return it.direction }

// Pass the Optimize() call along to the subiterator. If it becomes Null,
// then the HasA becomes Null (there are no triples that have any directions).
func (it *HasaIterator) Optimize() (Iterator, bool) {
	newPrimary, changed := it.primaryIt.Optimize()
	if changed {
		it.primaryIt = newPrimary
		if it.primaryIt.Type() == "null" {
			return it.primaryIt, true
		}
	}
	return it, false
}

// Pass the TagResults down the chain.
func (it *HasaIterator) TagResults(out *map[string]TSVal) {
	it.BaseIterator.TagResults(out)
	it.primaryIt.TagResults(out)
}

// DEPRECATED Return results in a ResultTree.
func (it *HasaIterator) GetResultTree() *ResultTree {
	tree := NewResultTree(it.LastResult())
	tree.AddSubtree(it.primaryIt.GetResultTree())
	return tree
}

// Print some information about this iterator.
func (it *HasaIterator) DebugString(indent int) string {
	var tags string
	for _, k := range it.Tags() {
		tags += fmt.Sprintf("%s;", k)
	}
	return fmt.Sprintf("%s(%s %d tags:%s direction:%s\n%s)", strings.Repeat(" ", indent), it.Type(), it.GetUid(), tags, it.direction, it.primaryIt.DebugString(indent+4))
}

// Check a value against our internal iterator. In order to do this, we must first open a new
// iterator of "triples that have `val` in our direction", given to us by the triple store,
// and then Next() values out of that iterator and Check() them against our subiterator.
func (it *HasaIterator) Check(val TSVal) bool {
	CheckLogIn(it, val)
	if glog.V(4) {
		glog.V(4).Infoln("Id is", it.ts.GetNameFor(val))
	}
	// TODO(barakmich): Optimize this
	if it.resultIt != nil {
		it.resultIt.Close()
	}
	it.resultIt = it.ts.GetTripleIterator(it.direction, val)
	return CheckLogOut(it, val, it.GetCheckResult())
}

// GetCheckResult() is shared code between Check() and GetNextResult() -- calls next on the
// result iterator (a triple iterator based on the last checked value) and returns true if
// another match is made.
func (it *HasaIterator) GetCheckResult() bool {
	for {
		linkVal, ok := it.resultIt.Next()
		if !ok {
			break
		}
		if glog.V(4) {
			glog.V(4).Infoln("Triple is", it.ts.GetTriple(linkVal).ToString())
		}
		if it.primaryIt.Check(linkVal) {
			it.Last = it.ts.GetTripleDirection(linkVal, it.direction)
			return true
		}
	}
	return false
}

// Get the next result that matches this branch.
func (it *HasaIterator) NextResult() bool {
	// Order here is important. If the subiterator has a NextResult, then we
	// need do nothing -- there is a next result, and we shouldn't move forward.
	// However, we then need to get the next result from our last Check().
	//
	// The upshot is, the end of NextResult() bubbles up from the bottom of the
	// iterator tree up, and we need to respect that.
	if it.primaryIt.NextResult() {
		return true
	}
	return it.GetCheckResult()
}

// Get the next result from this iterator. This is simpler than Check. We have a
// subiterator we can get a value from, and we can take that resultant triple,
// pull our direction out of it, and return that.
func (it *HasaIterator) Next() (TSVal, bool) {
	NextLogIn(it)
	if it.resultIt != nil {
		it.resultIt.Close()
	}
	it.resultIt = &NullIterator{}

	tID, ok := it.primaryIt.Next()
	if !ok {
		return NextLogOut(it, 0, false)
	}
	name := it.ts.GetTriple(tID).Get(it.direction)
	val := it.ts.GetIdFor(name)
	it.Last = val
	return NextLogOut(it, val, true)
}

// GetStats() returns the statistics on the HasA iterator. This is curious. Next
// cost is easy, it's an extra call or so on top of the subiterator Next cost.
// CheckCost involves going to the TripleStore, iterating out values, and hoping
// one sticks -- potentially expensive, depending on fanout. Size, however, is
// potentially smaller. we know at worst it's the size of the subiterator, but
// if there are many repeated values, it could be much smaller in totality.
func (it *HasaIterator) GetStats() *IteratorStats {
	subitStats := it.primaryIt.GetStats()
	// TODO(barakmich): These should really come from the triplestore itself
	// and be optimized.
	faninFactor := int64(1)
	fanoutFactor := int64(30)
	nextConstant := int64(2)
	tripleConstant := int64(1)
	return &IteratorStats{
		NextCost:  tripleConstant + subitStats.NextCost,
		CheckCost: (fanoutFactor * nextConstant) * subitStats.CheckCost,
		Size:      faninFactor * subitStats.Size,
	}
}

// Close the subiterator, the result iterator (if any) and the HasA.
func (it *HasaIterator) Close() {
	if it.resultIt != nil {
		it.resultIt.Close()
	}
	it.primaryIt.Close()
}

// Register this iterator as a HasA.
func (it *HasaIterator) Type() string { return "hasa" }
