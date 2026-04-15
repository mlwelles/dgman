/*
 * Copyright (C) 2026 Dolan and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dgman

import (
	stdjson "encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test types simulating modusgraph-gen output ---

// privateBook is an entity with unexported data fields (like what modusgraph-gen produces).
// UID and DType are exported (required for dgman UID writeback).
type privateBook struct {
	UID   string   `json:"uid,omitempty"`
	DType []string `json:"dgraph.type,omitempty" dgraph:"PrivateBook"`
	title string   `json:"title,omitempty" dgraph:"index=term"`
	year  int      `json:"year,omitempty"`
}

// privateBookReflectable is the all-exported mirror struct used for
// tag introspection and mutation payload via Reflectable.
type privateBookReflectable struct {
	UID   string   `json:"uid,omitempty"`
	DType []string `json:"dgraph.type,omitempty" dgraph:"PrivateBook"`
	Title string   `json:"title,omitempty" dgraph:"index=term"`
	Year  int      `json:"year,omitempty"`
}

// ToReflectable returns a populated all-exported copy for mutations
// and a type-carrier for query building.
func (b *privateBook) ToReflectable() any {
	return &privateBookReflectable{
		UID:   b.UID,
		DType: b.DType,
		Title: b.title,
		Year:  b.year,
	}
}

// FromReflectable copies UID/DType from the mutated reflectable back to the entity.
func (b *privateBook) FromReflectable(model any) {
	r := model.(*privateBookReflectable)
	b.UID = r.UID
	b.DType = r.DType
}

// UnmarshalJSON populates private fields from JSON (keyed by predicate name).
func (b *privateBook) UnmarshalJSON(data []byte) error {
	var raw map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["uid"]; ok {
		stdjson.Unmarshal(v, &b.UID)
	}
	if v, ok := raw["dgraph.type"]; ok {
		stdjson.Unmarshal(v, &b.DType)
	}
	if v, ok := raw["title"]; ok {
		stdjson.Unmarshal(v, &b.title)
	}
	if v, ok := raw["year"]; ok {
		stdjson.Unmarshal(v, &b.year)
	}
	return nil
}

// Accessors for test assertions.
func (b *privateBook) Title() string { return b.title }
func (b *privateBook) Year() int     { return b.year }

// --- Compile-time interface check ---
var _ HasReflectable = (*privateBook)(nil)

// --- Tests ---

func TestReflectableBasicRoundTrip(t *testing.T) {
	c := newDgraphClient()
	dropAll(c)
	defer dropAll(c)

	// Use the reflectable for schema creation (it has exported fields)
	_, err := CreateSchema(c, privateBookReflectable{})
	require.NoError(t, err)

	book := &privateBook{
		title: "Dune",
		year:  1965,
	}

	tx := NewTxn(c).SetCommitNow()
	uids, err := tx.MutateBasic(book)
	require.NoError(t, err)
	assert.NotEmpty(t, uids)
	assert.NotEmpty(t, book.UID, "UID should be written back via FromReflectable")
	assert.Contains(t, book.DType, "PrivateBook", "DType should be written back via FromReflectable")

	// Read it back using Reflectable (query path)
	var got privateBook
	err = NewReadOnlyTxn(c).Get(&got).UID(book.UID).Node()
	require.NoError(t, err)

	assert.Equal(t, book.UID, got.UID)
	assert.Equal(t, "Dune", got.Title(), "title should round-trip through private fields")
	assert.Equal(t, 1965, got.Year(), "year should round-trip through private fields")
}

func TestReflectableDoRoundTrip(t *testing.T) {
	c := newDgraphClient()
	dropAll(c)
	defer dropAll(c)

	// Need a unique field for the do() path. Can't add methods to local types,
	// so use the reflectable directly to verify the do() path works.
	type privateBookUniqueReflectable struct {
		UID   string   `json:"uid,omitempty"`
		DType []string `json:"dgraph.type,omitempty" dgraph:"PrivateBookUnique"`
		Title string   `json:"title,omitempty" dgraph:"index=term unique"`
		Year  int      `json:"year,omitempty"`
	}

	_, err := CreateSchema(c, privateBookUniqueReflectable{})
	require.NoError(t, err)

	book := &privateBookUniqueReflectable{
		Title: "Neuromancer",
		Year:  1984,
	}

	tx := NewTxn(c).SetCommitNow()
	uids, err := tx.Mutate(book)
	require.NoError(t, err)
	assert.NotEmpty(t, uids)
	assert.NotEmpty(t, book.UID)

	// Read back
	var got privateBookUniqueReflectable
	err = NewReadOnlyTxn(c).Get(&got).UID(book.UID).Node()
	require.NoError(t, err)
	assert.Equal(t, "Neuromancer", got.Title)
	assert.Equal(t, 1984, got.Year)
}

func TestReflectableNodesList(t *testing.T) {
	c := newDgraphClient()
	dropAll(c)
	defer dropAll(c)

	_, err := CreateSchema(c, privateBookReflectable{})
	require.NoError(t, err)

	// Insert two books via Reflectable
	book1 := &privateBook{title: "Foundation", year: 1951}
	book2 := &privateBook{title: "Snow Crash", year: 1992}

	tx1 := NewTxn(c).SetCommitNow()
	_, err = tx1.MutateBasic(book1)
	require.NoError(t, err)

	tx2 := NewTxn(c).SetCommitNow()
	_, err = tx2.MutateBasic(book2)
	require.NoError(t, err)

	// Query single node
	var single privateBook
	err = NewReadOnlyTxn(c).Get(&single).UID(book1.UID).Node()
	require.NoError(t, err)
	assert.Equal(t, "Foundation", single.Title())

	// Query multiple nodes
	var books []privateBook
	err = NewReadOnlyTxn(c).Get(&privateBook{}).Nodes(&books)
	require.NoError(t, err)
	assert.Len(t, books, 2)

	// Verify both books have their data
	titles := map[string]bool{}
	for _, b := range books {
		titles[b.Title()] = true
	}
	assert.True(t, titles["Foundation"])
	assert.True(t, titles["Snow Crash"])
}
