package quickbooks

import (
	"encoding/json"
	"strings"
	"time"
)

type ChangedEntityBase struct {
	StartPosition int64  `json:"startPosition"`
	MaxResults    int64  `json:"maxResult"`
	TotalCount    *int64 `json:"totalCount,omitempty"`
}

type DeletedEntity struct {
	ID       string `json:"Id"`
	Status   string `json:"status"`
	MetaData struct {
		LastUpdatedTime time.Time
	}
}

type MaybeDeleted[T any] struct {
	Deleted *DeletedEntity
	Entity  *T
}

func (m *MaybeDeleted[T]) UnmarshalJSON(data []byte) error {
	// peek at status field
	var peek struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return err
	}
	// QuickBooks sends "Deleted"; compare without case in case it ever changes.
	if strings.EqualFold(peek.Status, "deleted") {
		m.Deleted = &DeletedEntity{}
		return json.Unmarshal(data, m.Deleted)
	}
	m.Entity = new(T)
	return json.Unmarshal(data, m.Entity)
}
