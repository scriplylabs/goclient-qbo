package quickbooks

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDateJSON(t *testing.T) {
	got, err := json.Marshal(Date{time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)})
	require.NoError(t, err)
	assert.Equal(t, `"2026-10-05"`, string(got))

	// The date is written in the time's own location, never shifted to UTC.
	pacific := time.FixedZone("PDT", -7*60*60)
	got, err = json.Marshal(Date{time.Date(2026, 10, 5, 23, 30, 0, 0, pacific)})
	require.NoError(t, err)
	assert.Equal(t, `"2026-10-05"`, string(got))

	for _, in := range []string{`"2026-10-05"`, `"2026-10-05T00:00:00Z"`, `"2026-10-05T00:00:00-07:00"`, `"2026-10-05T00:00:00.123-07:00"`} {
		var d Date
		require.NoError(t, json.Unmarshal([]byte(in), &d), in)
		assert.Equal(t, "2026-10-05", d.Format("2006-01-02"), in)
	}

	// What Date writes, it reads back.
	var back Date
	require.NoError(t, json.Unmarshal(got, &back))
	assert.Equal(t, "2026-10-05", back.Format("2006-01-02"))
}

func TestDateTimeJSON(t *testing.T) {
	pacific := time.FixedZone("PDT", -7*60*60)
	start := DateTime{time.Date(2026, 10, 5, 9, 30, 0, 0, pacific)}

	got, err := json.Marshal(start)
	require.NoError(t, err)
	assert.Equal(t, `"2026-10-05T09:30:00-07:00"`, string(got))

	var back DateTime
	require.NoError(t, json.Unmarshal(got, &back))
	assert.True(t, start.Equal(back.Time))
	assert.Equal(t, "2026-10-05T09:30:00-07:00", back.String())

	// QuickBooks sends an authentication fault's time as epoch milliseconds.
	var ms DateTime
	require.NoError(t, json.Unmarshal([]byte(`1792694440314`), &ms))
	assert.Equal(t, int64(1792694440314), ms.UnixMilli())

	var date DateTime
	require.NoError(t, json.Unmarshal([]byte(`"2026-10-05"`), &date))
	assert.Equal(t, "2026-10-05", date.Format("2006-01-02"))

	var bad DateTime
	assert.Error(t, json.Unmarshal([]byte(`true`), &bad))
}

// TimeActivity keeps its time of day; MetaData keeps its timestamps.
func TestDateTimeFields(t *testing.T) {
	pacific := time.FixedZone("PDT", -7*60*60)
	start := DateTime{time.Date(2026, 10, 5, 9, 30, 0, 0, pacific)}
	end := DateTime{time.Date(2026, 10, 5, 17, 0, 0, 0, pacific)}
	day := Date{time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)}

	got, err := json.Marshal(TimeActivityCreateInput{NameOf: "Employee", TxnDate: &day, StartTime: &start, EndTime: &end})
	require.NoError(t, err)
	assert.JSONEq(t, `{"NameOf":"Employee","TxnDate":"2026-10-05","StartTime":"2026-10-05T09:30:00-07:00","EndTime":"2026-10-05T17:00:00-07:00"}`, string(got))

	var m MetaData
	require.NoError(t, json.Unmarshal([]byte(`{"CreateTime":"2026-10-22T11:40:29-07:00","LastUpdatedTime":"2026-10-22T11:41:10-07:00"}`), &m))
	got, err = json.Marshal(m)
	require.NoError(t, err)
	assert.JSONEq(t, `{"CreateTime":"2026-10-22T11:40:29-07:00","LastUpdatedTime":"2026-10-22T11:41:10-07:00"}`, string(got))
}
